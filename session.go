package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	serverLoggingSuffix = "SERVER"
	clientLoggingSuffix = "CLIENT"
)

var (
	errClosed = errors.New("session closed")
)

type parser interface {
	parse() (message, error)
}

type subscribeIDer interface {
	message
	subscribeID() uint64
}

type trackNamespacer interface {
	message
	trackNamespace() string
}

type trackKey struct {
	namespace string
	trackname string
}

type sessionInternals struct {
	logger                *slog.Logger
	serverHandshakeDoneCh chan struct{}
	controlStreamStoreCh  chan controlMessageSender // Needs to be buffered
	closeOnce             sync.Once
	closed                chan struct{}
	sendSubscriptions     *syncMap[uint64, *sendSubscription]
	receiveSubscriptions  *syncMap[uint64, *RemoteTrack]
	localAnnouncements    *syncMap[string, *Announcement]
	remoteAnnouncements   *syncMap[string, *Announcement]
	localTracks           *syncMap[trackKey, *LocalTrack]
}

func newSessionInternals(logSuffix string) *sessionInternals {
	return &sessionInternals{
		logger:                defaultLogger.WithGroup(fmt.Sprintf("MOQ_SESSION_%v", logSuffix)),
		serverHandshakeDoneCh: make(chan struct{}),
		controlStreamStoreCh:  make(chan controlMessageSender, 1),
		closeOnce:             sync.Once{},
		closed:                make(chan struct{}),
		sendSubscriptions:     newSyncMap[uint64, *sendSubscription](),
		receiveSubscriptions:  newSyncMap[uint64, *RemoteTrack](),
		localAnnouncements:    newSyncMap[string, *Announcement](),
		remoteAnnouncements:   newSyncMap[string, *Announcement](),
		localTracks:           newSyncMap[trackKey, *LocalTrack](),
	}
}

type controlMessageSender interface {
	enqueue(message)
	close()
}

type Session struct {
	Conn                Connection
	EnableDatagrams     bool
	LocalRole           Role
	RemoteRole          Role
	AnnouncementHandler AnnouncementHandler
	SubscriptionHandler SubscriptionHandler

	handshakeDone bool
	controlStream controlMessageSender
	isClient      bool
	si            *sessionInternals
}

func (s *Session) initRole() {
	switch s.LocalRole {
	case RolePublisher, RoleSubscriber, RolePubSub:
	default:
		s.LocalRole = RolePubSub
	}
	switch s.RemoteRole {
	case RolePublisher, RoleSubscriber, RolePubSub:
	default:
		s.RemoteRole = RolePubSub
	}
}

func (s *Session) validateRemoteRoleParameter(setupParameters parameters) error {
	remoteRoleParam, ok := setupParameters[roleParameterKey]
	if !ok {
		return s.CloseWithError(ErrorCodeProtocolViolation, "missing role parameter")
	}
	remoteRoleParamValue, ok := remoteRoleParam.(varintParameter)
	if !ok {
		return s.CloseWithError(ErrorCodeProtocolViolation, "invalid role parameter type")
	}
	switch Role(remoteRoleParamValue.V) {
	case RolePublisher, RoleSubscriber, RolePubSub:
		s.RemoteRole = Role(remoteRoleParamValue.V)
	default:
		return s.CloseWithError(ErrorCodeProtocolViolation, "invalid role parameter value")
	}
	return nil
}

func (s *Session) storeControlStream(cs controlMessageSender) {
	s.si.controlStreamStoreCh <- cs
}

func (s *Session) loadControlStream() controlMessageSender {
	return <-s.si.controlStreamStoreCh
}

func (s *Session) RunClient() error {
	s.si = newSessionInternals(clientLoggingSuffix)
	s.isClient = true
	s.initRole()
	controlStream, err := s.Conn.OpenStream()
	if err != nil {
		return err
	}
	s.controlStream = newControlStream(controlStream, s.handleControlMessage)
	s.controlStream.enqueue(&clientSetupMessage{
		SupportedVersions: []version{CURRENT_VERSION},
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				K: roleParameterKey,
				V: uint64(s.LocalRole),
			},
		},
	})
	go s.run()
	return nil
}

func (s *Session) RunServer(ctx context.Context) error {
	s.si = newSessionInternals(serverLoggingSuffix)
	s.isClient = false
	s.initRole()
	controlStream, err := s.Conn.AcceptStream(ctx)
	if err != nil {
		return err
	}
	s.si.controlStreamStoreCh <- newControlStream(controlStream, s.handleControlMessage)
	select {
	case <-ctx.Done():
		s.Close()
		return ctx.Err()
	case <-s.si.serverHandshakeDoneCh:
	}
	s.si.logger.Info("server handshake done")
	go s.run()
	return nil
}

func (s *Session) initClient(setup *serverSetupMessage) error {
	if setup.SelectedVersion != CURRENT_VERSION {
		return s.CloseWithError(ErrorCodeUnsupportedVersion, "unsupported version")
	}
	if err := s.validateRemoteRoleParameter(setup.SetupParameters); err != nil {
		return err
	}
	s.handshakeDone = true
	return nil
}

func (s *Session) initServer(setup *clientSetupMessage) error {
	s.controlStream = s.loadControlStream()
	if !slices.Contains(setup.SupportedVersions, CURRENT_VERSION) {
		return s.CloseWithError(ErrorCodeUnsupportedVersion, "unsupported version")
	}
	if err := s.validateRemoteRoleParameter(setup.SetupParameters); err != nil {
		return err
	}
	ssm := &serverSetupMessage{
		SelectedVersion: CURRENT_VERSION,
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				K: roleParameterKey,
				V: uint64(RolePubSub),
			},
		},
	}
	s.controlStream.enqueue(ssm)
	s.handshakeDone = true
	close(s.si.serverHandshakeDoneCh)
	return nil
}

func (s *Session) run() {
	go s.acceptUnidirectionalStreams()
	if s.EnableDatagrams {
		go s.acceptDatagrams()
	}
}

func (s *Session) acceptUnidirectionalStream() (ReceiveStream, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.si.closed:
			cancel()
		case <-ctx.Done():
		}
	}()
	return s.Conn.AcceptUniStream(ctx)
}

func (s *Session) acceptUnidirectionalStreams() {
	for {
		stream, err := s.acceptUnidirectionalStream()
		if err != nil {
			s.si.logger.Error("failed to accept uni stream", "error", err)
			s.peerClosed()
			return
		}
		go s.handleIncomingUniStream(stream)
	}
}

func (s *Session) handleIncomingUniStream(stream ReceiveStream) {
	p := newParser(quicvarint.NewReader(stream))
	msg, err := p.parse()
	if err != nil {
		s.si.logger.Error("failed to parse message", "error", err)
		return
	}
	switch h := msg.(type) {
	case *objectMessage:
		sub, ok := s.si.receiveSubscriptions.get(h.SubscribeID)
		if !ok {
			s.si.logger.Warn("got object for unknown subscribe ID")
			return
		}
		sub.push(Object{
			GroupID:              h.GroupID,
			ObjectID:             h.ObjectID,
			ObjectSendOrder:      0,
			ForwardingPreference: ObjectForwardingPreferenceStream,
			Payload:              h.ObjectPayload,
		})
	case *streamHeaderTrackMessage:
		sub, ok := s.si.receiveSubscriptions.get(h.SubscribeID)
		if !ok {
			s.si.logger.Warn("got stream header track message for unknown subscription")
			return
		}
		sub.readTrackHeaderStream(stream)
	case *streamHeaderGroupMessage:
		sub, ok := s.si.receiveSubscriptions.get(h.SubscribeID)
		if !ok {
			s.si.logger.Warn("got stream header track message for unknown subscription")
			return
		}
		sub.readGroupHeaderStream(stream, h.GroupID)
	}
}

func (s *Session) acceptDatagram() ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.si.closed:
			cancel()
		case <-ctx.Done():
		}
	}()
	return s.Conn.ReceiveDatagram(ctx)
}

func (s *Session) acceptDatagrams() {
	for {
		dgram, err := s.acceptDatagram()
		if err != nil {
			s.si.logger.Error("failed to receive datagram", "error", err)
			s.peerClosed()
			return
		}
		go s.readObjectMessages(bytes.NewReader(dgram))
	}
}

func (s *Session) readObjectMessages(r messageReader) {
	msgParser := newParser(r)
	for {
		msg, err := msgParser.parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			s.si.logger.Error("failed to parse message", "error", err)
			pe := &ProtocolError{
				code:    ErrorCodeProtocolViolation,
				message: "invalid message format",
			}
			_ = s.Conn.CloseWithError(pe.code, pe.message)
			return
		}
		o, ok := msg.(*objectMessage)
		if !ok {
			pe := &ProtocolError{
				code:    ErrorCodeProtocolViolation,
				message: "received unexpected control message on object stream or datagram",
			}
			// TODO: Set error on session to surface to application?
			_ = s.Conn.CloseWithError(pe.code, pe.message)
			return
		}
		sub, ok := s.si.receiveSubscriptions.get(o.SubscribeID)
		if ok {
			sub.push(Object{
				GroupID:              o.GroupID,
				ObjectID:             o.ObjectID,
				ObjectSendOrder:      0,
				ForwardingPreference: ObjectForwardingPreferenceDatagram,
				Payload:              o.ObjectPayload,
			})
		}
		s.si.logger.Warn("dropping object message for unknown track")
	}
}

func (s *Session) handleControlMessage(msg message) error {
	s.si.logger.Info("received control message", "type", fmt.Sprintf("%T", msg), "message", msg)
	if s.handshakeDone {
		if err := s.handleNonSetupMessage(msg); err != nil {
			return err
		}
		return nil
	}
	switch mt := msg.(type) {
	case *serverSetupMessage:
		return s.initClient(mt)
	case *clientSetupMessage:
		return s.initServer(mt)
	}
	s.si.logger.Info("received message during handshake", "message", msg)
	pe := ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "received unexpected first message on control stream",
	}
	s.controlStream.close()
	_ = s.Conn.CloseWithError(pe.code, pe.message)
	return pe
}

func (s *Session) handleNonSetupMessage(msg message) error {
	switch m := msg.(type) {
	case *subscribeMessage:
		s.handleSubscribe(m)
	case *subscribeUpdateMessage:
		panic("TODO")
	case *subscribeOkMessage:
		return s.handleSubscriptionResponse(m)
	case *subscribeErrorMessage:
		return s.handleSubscriptionResponse(m)
	case *subscribeDoneMessage:
		s.handleSubscribeDone(m)
	case *unsubscribeMessage:
		return s.handleUnsubscribe(m)
	case *announceMessage:
		s.handleAnnounceMessage(m)
	case *announceOkMessage:
		return s.handleAnnouncementResponse(m)
	case *announceErrorMessage:
		return s.handleAnnouncementResponse(m)
	case *goAwayMessage:
		panic("TODO")
	default:
		return &ProtocolError{
			code:    ErrorCodeInternal,
			message: "received unexpected message type on control stream",
		}
	}
	return nil
}

func (s *Session) handleSubscriptionResponse(msg subscribeIDer) error {
	sub, ok := s.si.receiveSubscriptions.get(msg.subscribeID())
	if !ok {
		return &ProtocolError{
			code:    ErrorCodeInternal,
			message: "received subscription response message to an unknown subscription",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case sub.responseCh <- msg:
	case <-s.si.closed:
		return errClosed
	}
	return nil
}

func (s *Session) handleAnnouncementResponse(msg trackNamespacer) error {
	a, ok := s.si.localAnnouncements.get(msg.trackNamespace())
	if !ok {
		return &ProtocolError{
			code:    ErrorCodeInternal,
			message: "received announcement response message to an unknown announcement",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case a.responseCh <- msg:
	case <-s.si.closed:
		return errClosed
	}
	return nil
}

func (s *Session) subscribeToLocalTrack(sub *Subscription, t *LocalTrack) {
	sendSub := newSendSubscription(s.Conn, sub.ID, sub.TrackAlias, sub.Namespace, sub.TrackName)
	if err := s.si.sendSubscriptions.add(sub.ID, sendSub); err != nil {
		s.controlStream.enqueue(&subscribeErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    ErrorCodeInternal, // TODO: Set better error code?
			ReasonPhrase: err.Error(),
			TrackAlias:   sub.TrackAlias,
		})
		return
	}
	id, err := t.subscribe(sendSub)
	if err != nil {
		s.controlStream.enqueue(&subscribeErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    ErrorCodeInternal, // TODO: Set better error code?
			ReasonPhrase: err.Error(),
			TrackAlias:   sub.TrackAlias,
		})
		return
	}
	sendSub.subscriptionIDinTrack = id
	s.controlStream.enqueue(&subscribeOkMessage{
		SubscribeID:   sub.ID,
		Expires:       0,     // TODO
		ContentExists: false, // TODO
		FinalGroup:    0,     // TODO
		FinalObject:   0,     // TODO
	})
}

func (s *Session) rejectSubscription(sub *Subscription, code uint64, reason string) {
	s.controlStream.enqueue(&subscribeErrorMessage{
		SubscribeID:  sub.ID,
		ErrorCode:    code,
		ReasonPhrase: reason,
		TrackAlias:   sub.TrackAlias,
	})
}

func (s *Session) handleSubscribe(msg *subscribeMessage) {
	var authValue string
	auth, ok := msg.Parameters[authorizationParameterKey]
	authString, isStringParam := auth.(stringParameter)
	if ok && isStringParam {
		authValue = authString.V
	}
	sub := &Subscription{
		ID:            msg.SubscribeID,
		TrackAlias:    msg.TrackAlias,
		Namespace:     msg.TrackNamespace,
		TrackName:     msg.TrackName,
		Authorization: authValue,
	}
	t, ok := s.si.localTracks.get(trackKey{
		namespace: msg.TrackNamespace,
		trackname: msg.TrackName,
	})
	if ok {
		s.subscribeToLocalTrack(sub, t)
		return
	}
	if s.SubscriptionHandler != nil {
		s.SubscriptionHandler.HandleSubscription(s, sub, &defaultSubscriptionResponseWriter{
			subscription: sub,
			session:      s,
		})
		return
	}
	s.rejectSubscription(sub, ErrorCodeTrackNotFound, "track not found")
}

func (s *Session) handleUnsubscribe(msg *unsubscribeMessage) error {
	sub, ok := s.si.sendSubscriptions.get(msg.SubscribeID)
	if !ok {
		return errors.New("subscription not found")
	}
	track, ok := s.si.localTracks.get(trackKey{
		namespace: sub.namespace,
		trackname: sub.trackname,
	})
	if !ok {
		return errors.New("no track related to subscription found")
	}
	track.unsubscribe(sub.subscriptionIDinTrack)
	if err := sub.Close(); err != nil {
		panic(err)
	}
	s.si.sendSubscriptions.delete(msg.SubscribeID)
	s.controlStream.enqueue(&subscribeDoneMessage{
		SusbcribeID:   msg.SubscribeID,
		StatusCode:    0,
		ReasonPhrase:  "unsubscribed",
		ContentExists: false, // TODO
		FinalGroup:    0,     // TODO
		FinalObject:   0,     // TODO
	})
	return nil
}

func (s *Session) handleSubscribeDone(msg *subscribeDoneMessage) {
	sub, ok := s.si.receiveSubscriptions.get(msg.SusbcribeID)
	if !ok {
		s.si.logger.Info("got SubscribeDone for unknown subscription")
		return
	}
	sub.close()
	s.si.receiveSubscriptions.delete(msg.SusbcribeID)
}

func (s *Session) handleAnnounceMessage(msg *announceMessage) {
	a := &Announcement{
		responseCh: make(chan trackNamespacer),
		namespace:  msg.TrackNamespace,
		parameters: msg.TrackRequestParameters,
	}
	if err := s.si.remoteAnnouncements.add(a.namespace, a); err != nil {
		s.si.logger.Error("dropping announcement", "error", err)
		return
	}
	if s.AnnouncementHandler != nil {
		go s.AnnouncementHandler.HandleAnnouncement(s, a, &defaultAnnouncementResponseWriter{
			announcement: a,
			session:      s,
		})
	}
}

func (s *Session) rejectAnnouncement(a *Announcement, code uint64, reason string) {
	s.si.remoteAnnouncements.delete(a.namespace)
	s.controlStream.enqueue(&announceErrorMessage{
		TrackNamespace: a.namespace,
		ErrorCode:      code,
		ReasonPhrase:   reason,
	})
}

func (s *Session) acceptAnnouncement(a *Announcement) {
	s.controlStream.enqueue(&announceOkMessage{
		TrackNamespace: a.namespace,
	})
}

func (s *Session) unsubscribe(id uint64) {
	s.controlStream.enqueue(&unsubscribeMessage{
		SubscribeID: id,
	})
}

func (s *Session) peerClosed() {
	s.si.logger.Info("peerClosed called")
	s.si.closeOnce.Do(func() {
		close(s.si.closed)
		s.controlStream.close()
	})
}

func (s *Session) CloseWithError(code uint64, msg string) error {
	s.peerClosed()
	return s.Conn.CloseWithError(code, msg)
}

func (s *Session) Close() error {
	return s.CloseWithError(0, "")
}

func (s *Session) AddLocalTrack(t *LocalTrack) error {
	return s.si.localTracks.add(trackKey{
		namespace: t.Namespace,
		trackname: t.Name,
	}, t)
}

func (s *Session) Subscribe(ctx context.Context, subscribeID, trackAlias uint64, namespace, trackname string, auth string) (*RemoteTrack, error) {
	sm := &subscribeMessage{
		SubscribeID:    subscribeID,
		TrackAlias:     trackAlias,
		TrackNamespace: namespace,
		TrackName:      trackname,
		FilterType:     0,
		StartGroup:     0,
		StartObject:    0,
		EndGroup:       0,
		EndObject:      0,
		Parameters:     map[uint64]parameter{},
	}
	if len(auth) > 0 {
		sm.Parameters[authorizationParameterKey] = stringParameter{
			K: authorizationParameterKey,
			V: auth,
		}
	}
	sub := newRemoteTrack(sm.SubscribeID, s)
	if err := s.si.receiveSubscriptions.add(sm.SubscribeID, sub); err != nil {
		return nil, err
	}
	s.controlStream.enqueue(sm)
	var resp subscribeIDer
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.si.closed:
		return nil, errClosed
	case resp = <-sub.responseCh:
	}
	if resp.subscribeID() != sm.SubscribeID {
		// Should never happen, because messages are routed based on subscribe
		// ID. Wrong IDs would thus never end up here.
		s.si.logger.Error("internal error: received response message for wrong subscription ID", "expected_id", sm.SubscribeID, "repsonse_id", resp.subscribeID())
		return nil, errors.New("internal error: received response message for wrong subscription ID")
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		return sub, nil
	case *subscribeErrorMessage:
		s.si.receiveSubscriptions.delete(sm.SubscribeID)
		return nil, ApplicationError{
			code:   v.ErrorCode,
			mesage: v.ReasonPhrase,
		}
	}
	// Should never happen, because only subscribeMessage, subscribeOkMessage
	// and susbcribeErrorMessage implement the SubscribeIDer interface and
	// subscribeMessages should not be routed to this method.
	return nil, errors.New("received unexpected response message type to subscribeRequestMessage")
}

func (s *Session) Announce(ctx context.Context, namespace string) error {
	if len(namespace) == 0 {
		return errors.New("invalid track namespace")
	}
	am := &announceMessage{
		TrackNamespace:         namespace,
		TrackRequestParameters: map[uint64]parameter{},
	}
	responseCh := make(chan trackNamespacer)
	a := &Announcement{
		responseCh: responseCh,
	}
	if err := s.si.localAnnouncements.add(am.TrackNamespace, a); err != nil {
		return err
	}
	s.controlStream.enqueue(am)
	var resp trackNamespacer
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.si.closed:
		return errClosed
	case resp = <-responseCh:
	}
	if resp.trackNamespace() != am.TrackNamespace {
		// Should never happen, because messages are routed based on trackname.
		// Wrong tracknames would thus never end up here.
		s.si.logger.Error("internal error: received response message for wrong announce track namespace", "expected_track_namespace", am.TrackNamespace, "response_track_namespace", resp.trackNamespace())
		return errors.New("internal error: received response message for wrong announce track namespace")
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		return nil
	case *announceErrorMessage:
		return ApplicationError{
			code:   v.ErrorCode,
			mesage: v.ReasonPhrase,
		}
	}
	// Should never happen, because only announceMessage, announceOkMessage
	// and announceErrorMessage implement the trackNamespacer interface and
	// announceMessages should not be routed to this method.
	return errors.New("received unexpected response message type to announceMessage")
}
