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

	"github.com/mengelbart/moqtransport/internal/wire"
)

const (
	serverLoggingSuffix = "SERVER"
	clientLoggingSuffix = "CLIENT"
)

var (
	errClosed = errors.New("session closed")
)

type subscribeIDer interface {
	wire.Message
	GetSubscribeID() uint64
}

type trackNamespacer interface {
	wire.Message
	GetTrackNamespace() string
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
	enqueue(wire.Message)
	close()
}

type Session struct {
	Conn                Connection
	EnableDatagrams     bool
	LocalRole           Role
	RemoteRole          Role
	AnnouncementHandler AnnouncementHandler
	SubscriptionHandler SubscriptionHandler
	Path                string

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

func (s *Session) validateRemoteRoleParameter(setupParameters wire.Parameters) error {
	remoteRoleParam, ok := setupParameters[wire.RoleParameterKey]
	if !ok {
		return s.CloseWithError(ErrorCodeProtocolViolation, "missing role parameter")
	}
	remoteRoleParamValue, ok := remoteRoleParam.(*wire.VarintParameter)
	if !ok {
		return s.CloseWithError(ErrorCodeProtocolViolation, "invalid role parameter type")
	}
	switch wire.Role(remoteRoleParamValue.Value) {
	case wire.RolePublisher, wire.RoleSubscriber, wire.RolePubSub:
		s.RemoteRole = wire.Role(remoteRoleParamValue.Value)
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
	s.controlStream.enqueue(&wire.ClientSetupMessage{
		SupportedVersions: []wire.Version{wire.CurrentVersion},
		SetupParameters: wire.Parameters{
			wire.RoleParameterKey: wire.VarintParameter{
				Type:  wire.RoleParameterKey,
				Value: uint64(s.LocalRole),
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
	s.storeControlStream(newControlStream(controlStream, s.handleControlMessage))
	select {
	case <-ctx.Done():
		s.si.logger.Error("context done before control stream handshake done")
		s.Close()
		return ctx.Err()
	case <-s.si.serverHandshakeDoneCh:
	}
	s.si.logger.Info("server handshake done")
	go s.run()
	return nil
}

func (s *Session) initClient(setup *wire.ServerSetupMessage) error {
	if setup.SelectedVersion != wire.CurrentVersion {
		s.si.logger.Error("unsupported version", "remote_server_supported_version", setup.SelectedVersion, "client_supported_version", wire.CurrentVersion)
		return s.CloseWithError(ErrorCodeUnsupportedVersion, "unsupported version")
	}
	if err := s.validateRemoteRoleParameter(setup.SetupParameters); err != nil {
		s.si.logger.Error("failed to validate remote role parameter", "error", err)
		return err
	}
	s.handshakeDone = true
	return nil
}

func (s *Session) initServer(setup *wire.ClientSetupMessage) error {
	s.controlStream = s.loadControlStream()
	if !slices.Contains(setup.SupportedVersions, wire.CurrentVersion) {
		s.si.logger.Error("unsupported version", "remote_client_supported_versions", setup.SupportedVersions, "server_supported_version", wire.CurrentVersion)
		return s.CloseWithError(ErrorCodeUnsupportedVersion, "unsupported version")
	}
	if err := s.validateRemoteRoleParameter(setup.SetupParameters); err != nil {
		s.si.logger.Error("failed to validate remote role parameter", "error", err)
		return err
	}
	pathParam, ok := setup.SetupParameters[wire.PathParameterKey]
	if ok {
		pathParamValue, ok := pathParam.(*wire.StringParameter)
		if !ok {
			return s.CloseWithError(ErrorCodeProtocolViolation, "invalid path parameter type")
		}
		s.Path = pathParamValue.Value
	}
	ssm := &wire.ServerSetupMessage{
		SelectedVersion: wire.CurrentVersion,
		SetupParameters: wire.Parameters{
			wire.RoleParameterKey: &wire.VarintParameter{
				Type:  wire.RoleParameterKey,
				Value: uint64(wire.RolePubSub),
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
	p := wire.NewObjectStreamParser(stream)
	msg, err := p.Parse()
	if err != nil {
		s.si.logger.Error("failed to parse message", "error", err)
		return
	}
	sub, ok := s.si.receiveSubscriptions.get(msg.SubscribeID)
	if !ok {
		s.si.logger.Warn("got object for unknown subscribe ID")
		return
	}
	sub.push(Object{
		GroupID:              msg.GroupID,
		ObjectID:             msg.ObjectID,
		PublisherPriority:    msg.PublisherPriority,
		ForwardingPreference: objectForwardingPreferenceFromMessageType(msg.Type),
		Payload:              msg.ObjectPayload,
	})
	sub.readObjectStream(p)
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
		go s.readObjectMessage(bytes.NewReader(dgram))
	}
}

func (s *Session) readObjectMessage(r io.Reader) {
	msgParser := wire.NewObjectStreamParser(r)
	o, err := msgParser.Parse()
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
	sub, ok := s.si.receiveSubscriptions.get(o.SubscribeID)
	if !ok {
		s.si.logger.Warn("dropping object message for unknown track")
		return
	}
	sub.push(Object{
		GroupID:              o.GroupID,
		ObjectID:             o.ObjectID,
		PublisherPriority:    o.PublisherPriority,
		ForwardingPreference: ObjectForwardingPreferenceDatagram,
		Payload:              o.ObjectPayload,
	})
}

func (s *Session) handleControlMessage(msg wire.Message) error {
	s.si.logger.Info("received control message", "type", fmt.Sprintf("%T", msg), "message", msg)
	if s.handshakeDone {
		if err := s.handleNonSetupMessage(msg); err != nil {
			return err
		}
		return nil
	}
	switch mt := msg.(type) {
	case *wire.ServerSetupMessage:
		return s.initClient(mt)
	case *wire.ClientSetupMessage:
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

func (s *Session) handleNonSetupMessage(msg wire.Message) error {
	switch m := msg.(type) {
	case *wire.SubscribeMessage:
		s.handleSubscribe(m)
	case *wire.SubscribeUpdateMessage:
		panic("TODO")
	case *wire.SubscribeOkMessage:
		return s.handleSubscriptionResponse(m)
	case *wire.SubscribeErrorMessage:
		return s.handleSubscriptionResponse(m)
	case *wire.AnnounceMessage:
		s.handleAnnounceMessage(m)
	case *wire.AnnounceOkMessage:
		return s.handleAnnouncementResponse(m)
	case *wire.AnnounceErrorMessage:
		return s.handleAnnouncementResponse(m)
	case *wire.UnannounceMessage:
		panic("TODO")
	case *wire.UnsubscribeMessage:
		return s.handleUnsubscribe(m)
	case *wire.SubscribeDoneMessage:
		s.handleSubscribeDone(m)
	case *wire.AnnounceCancelMessage:
		panic("TODO")
	case *wire.TrackStatusRequestMessage:
		panic("TODO")
	case *wire.TrackStatusMessage:
		panic("TODO")
	case *wire.GoAwayMessage:
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
	sub, ok := s.si.receiveSubscriptions.get(msg.GetSubscribeID())
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
	a, ok := s.si.localAnnouncements.get(msg.GetTrackNamespace())
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
		s.controlStream.enqueue(&wire.SubscribeErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    ErrorCodeInternal, // TODO: Set better error code?
			ReasonPhrase: err.Error(),
			TrackAlias:   sub.TrackAlias,
		})
		s.si.logger.Error("failed to save subscription", "error", err)
		return
	}
	id, err := t.subscribe(sendSub)
	if err != nil {
		s.controlStream.enqueue(&wire.SubscribeErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    ErrorCodeInternal, // TODO: Set better error code?
			ReasonPhrase: err.Error(),
			TrackAlias:   sub.TrackAlias,
		})
		s.si.logger.Error("failed to subscribe to track", "error", err)
		return
	}
	sendSub.subscriptionIDinTrack = id
	s.controlStream.enqueue(&wire.SubscribeOkMessage{
		SubscribeID:   sub.ID,
		Expires:       0,     // TODO
		GroupOrder:    1,     // TODO
		ContentExists: false, // TODO
		FinalGroup:    0,     // TODO
		FinalObject:   0,     // TODO
	})
}

func (s *Session) rejectSubscription(sub *Subscription, code uint64, reason string) {
	s.controlStream.enqueue(&wire.SubscribeErrorMessage{
		SubscribeID:  sub.ID,
		ErrorCode:    code,
		ReasonPhrase: reason,
		TrackAlias:   sub.TrackAlias,
	})
}

func (s *Session) handleSubscribe(msg *wire.SubscribeMessage) {
	var authValue string
	auth, ok := msg.Parameters[wire.AuthorizationParameterKey]
	authString, isStringParam := auth.(*wire.StringParameter)
	if ok && isStringParam {
		authValue = authString.Value
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

func (s *Session) handleUnsubscribe(msg *wire.UnsubscribeMessage) error {
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
	s.controlStream.enqueue(&wire.SubscribeDoneMessage{
		SubscribeID:   msg.SubscribeID,
		StatusCode:    0,
		ReasonPhrase:  "unsubscribed",
		ContentExists: false, // TODO
		FinalGroup:    0,     // TODO
		FinalObject:   0,     // TODO
	})
	return nil
}

func (s *Session) handleSubscribeDone(msg *wire.SubscribeDoneMessage) {
	sub, ok := s.si.receiveSubscriptions.get(msg.SubscribeID)
	if !ok {
		s.si.logger.Info("got SubscribeDone for unknown subscription")
		return
	}
	sub.close()
	s.si.receiveSubscriptions.delete(msg.SubscribeID)
}

func (s *Session) handleAnnounceMessage(msg *wire.AnnounceMessage) {
	a := &Announcement{
		responseCh: make(chan trackNamespacer),
		namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
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
	s.controlStream.enqueue(&wire.AnnounceErrorMessage{
		TrackNamespace: a.namespace,
		ErrorCode:      code,
		ReasonPhrase:   reason,
	})
}

func (s *Session) acceptAnnouncement(a *Announcement) {
	s.controlStream.enqueue(&wire.AnnounceOkMessage{
		TrackNamespace: a.namespace,
	})
}

func (s *Session) unsubscribe(id uint64) {
	s.controlStream.enqueue(&wire.UnsubscribeMessage{
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
	s.si.logger.Info("CloseWithError called", "code", code, "msg", msg)
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
	sm := &wire.SubscribeMessage{
		SubscribeID:    subscribeID,
		TrackAlias:     trackAlias,
		TrackNamespace: namespace,
		TrackName:      trackname,
		FilterType:     0,
		StartGroup:     0,
		StartObject:    0,
		EndGroup:       0,
		EndObject:      0,
		Parameters:     wire.Parameters{},
	}
	if len(auth) > 0 {
		sm.Parameters[wire.AuthorizationParameterKey] = &wire.StringParameter{
			Type:  wire.AuthorizationParameterKey,
			Value: auth,
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
	if resp.GetSubscribeID() != sm.SubscribeID {
		// Should never happen, because messages are routed based on subscribe
		// ID. Wrong IDs would thus never end up here.
		s.si.logger.Error("internal error: received response message for wrong subscription ID", "expected_id", sm.SubscribeID, "repsonse_id", resp.GetSubscribeID())
		return nil, errors.New("internal error: received response message for wrong subscription ID")
	}
	switch v := resp.(type) {
	case *wire.SubscribeOkMessage:
		return sub, nil
	case *wire.SubscribeErrorMessage:
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
	am := &wire.AnnounceMessage{
		TrackNamespace: namespace,
		Parameters:     wire.Parameters{},
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
	if resp.GetTrackNamespace() != am.TrackNamespace {
		// Should never happen, because messages are routed based on trackname.
		// Wrong tracknames would thus never end up here.
		s.si.logger.Error("internal error: received response message for wrong announce track namespace", "expected_track_namespace", am.TrackNamespace, "response_track_namespace", resp.GetTrackNamespace())
		return errors.New("internal error: received response message for wrong announce track namespace")
	}
	switch v := resp.(type) {
	case *wire.AnnounceOkMessage:
		return nil
	case *wire.AnnounceErrorMessage:
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
