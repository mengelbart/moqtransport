package moqtransport

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"slices"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

var (
	// ErrControlMessageQueueOverflow is returned if a control message cannot be
	// send due to queue overflow.
	ErrControlMessageQueueOverflow = errors.New("control message overflow, message not queued")

	ErrUnknownAnnouncementNamespace = errors.New("unknown announcement namespace")
)

var errMaxSubscribeIDViolated = errors.New("max subscribe ID violated")

type messageHandler interface {
	handle(*Message)
}

// A Session is an endpoint of a MoQ Session session.
type Session struct {
	logger *slog.Logger

	ctx       context.Context
	cancelCtx context.CancelCauseFunc

	handshakeDoneCh chan struct{}

	controlMessageSender controlMessageSender

	handler messageHandler

	version     wire.Version
	protocol    Protocol
	perspective Perspective
	path        string

	maxSubscribeID uint64

	outgoingAnnouncements *announcementMap
	incomingAnnouncements *announcementMap

	pendingOutgointAnnouncementSubscriptions *announcementSubscriptionMap
	pendingIncomingAnnouncementSubscriptions *announcementSubscriptionMap

	highestSubscribesBlocked atomic.Uint64
	remoteTracks             *remoteTrackMap
	localTracks              *localTrackMap
}

func (s *Session) remoteTrackBySubscribeID(id uint64) (*RemoteTrack, bool) {
	sub, ok := s.remoteTracks.findBySubscribeID(id)
	return sub, ok
}

func (s *Session) remoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	sub, ok := s.remoteTracks.findByTrackAlias(alias)
	return sub, ok
}

func (s *Session) receive(msg wire.ControlMessage) error {
	s.logger.Info("received message", "type", msg.Type().String(), "msg", msg)
	if s.handshakeDone() {
		var err error
		switch m := msg.(type) {
		case *wire.AnnounceMessage:
			err = s.onAnnounce(m)
		case *wire.UnannounceMessage:
			err = s.onUnannounce(m)
		case *wire.AnnounceOkMessage:
			err = s.onAnnounceOk(m)
		case *wire.AnnounceErrorMessage:
			err = s.onAnnounceError(m)
		case *wire.SubscribeOkMessage:
			err = s.onSubscribeOk(m)
		case *wire.FetchOkMessage:
			err = s.onFetchOk(m)
		case *wire.MaxSubscribeIDMessage:
			err = s.onMaxSubscribeID(m)
		case *wire.SubscribeDoneMessage:
			err = s.onSubscribeDone(m)
		case *wire.FetchMessage:
			err = s.onFetch(m)
		case *wire.FetchCancelMessage:
			err = s.onFetchCancel(m)
		case *wire.FetchErrorMessage:
			err = s.onFetchError(m)
		case *wire.AnnounceCancelMessage:
			err = s.onAnnounceCancel(m)
		case *wire.GoAwayMessage:
			err = s.onGoAway(m)
		case *wire.SubscribeAnnouncesErrorMessage:
			err = s.onSubscribeAnnouncesError(m)
		case *wire.SubscribeAnnouncesMessage:
			err = s.onSubscribeAnnounces(m)
		case *wire.SubscribeAnnouncesOkMessage:
			err = s.onSubscribeAnnouncesOk(m)
		case *wire.SubscribeErrorMessage:
			err = s.onSubscribeError(m)
		case *wire.SubscribeMessage:
			err = s.onSubscribe(m)
		case *wire.SubscribeUpdateMessage:
			err = s.onSubscribeUpdate(m)
		case *wire.SubscribesBlockedMessage:
			err = s.onSubscribesBlocked(m)
		case *wire.TrackStatusMessage:
			err = s.onTrackStatus(m)
		case *wire.TrackStatusRequestMessage:
			err = s.onTrackStatusRequest(m)
		case *wire.UnsubscribeAnnouncesMessage:
			err = s.onUnsubscribeAnnounces(m)
		case *wire.UnsubscribeMessage:
			err = s.onUnsubscribe(m)
		default:
			err = ProtocolError{
				code:    ErrorCodeProtocolViolation,
				message: "unexpected message type",
			}
		}
		return err
	}

	switch m := msg.(type) {
	case *wire.ClientSetupMessage:
		return s.onClientSetup(m)
	case *wire.ServerSetupMessage:
		return s.onServerSetup(m)
	}

	return ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unexpected message type before setup",
	}
}

func (s *Session) onClientSetup(m *wire.ClientSetupMessage) error {
	if s.perspective != PerspectiveServer {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "client received client setup message",
		}
	}
	selectedVersion := -1
	for _, v := range slices.Backward(wire.SupportedVersions) {
		if slices.Contains(m.SupportedVersions, v) {
			selectedVersion = int(v)
			break
		}
	}
	if selectedVersion == -1 {
		return ProtocolError{
			code:    ErrorCodeUnsupportedVersion,
			message: "incompatible versions",
		}
	}
	s.version = wire.Version(selectedVersion)

	path, err := validatePathParameter(m.SetupParameters, s.protocol == ProtocolQUIC)
	if err != nil {
		return err
	}
	s.path = path

	remoteMaxSubscribeID, err := validateMaxSubscribeIDParameter(m.SetupParameters)
	if err != nil {
		return err
	}
	if remoteMaxSubscribeID > 0 {
		if err = s.remoteTracks.updateMaxSubscribeID(remoteMaxSubscribeID); err != nil {
			return err
		}
	}

	if err := s.controlMessageSender.queueControlMessage(&wire.ServerSetupMessage{
		SelectedVersion: wire.Version(selectedVersion),
		SetupParameters: map[uint64]wire.Parameter{
			wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
				Type:  wire.MaxSubscribeIDParameterKey,
				Value: 100,
			},
		},
	}); err != nil {
		return err
	}
	close(s.handshakeDoneCh)
	return nil
}

func (s *Session) onServerSetup(m *wire.ServerSetupMessage) (err error) {
	if s.perspective != PerspectiveClient {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "server received server setup message",
		}
	}

	if !slices.Contains(wire.SupportedVersions, m.SelectedVersion) {
		return ProtocolError{
			code:    ErrorCodeUnsupportedVersion,
			message: "incompatible versions",
		}
	}
	s.version = m.SelectedVersion

	remoteMaxSubscribeID, err := validateMaxSubscribeIDParameter(m.SetupParameters)
	if err != nil {
		return err
	}
	if err = s.remoteTracks.updateMaxSubscribeID(remoteMaxSubscribeID); err != nil {
		return err
	}
	close(s.handshakeDoneCh)
	return nil
}

func (s *Session) onAnnounce(msg *wire.AnnounceMessage) error {
	a := &announcement{
		Namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
	}
	if err := s.incomingAnnouncements.add(a); err != nil {
		return ProtocolError{
			code:    ErrorCodeAnnouncementInternalError,
			message: err.Error(),
		}
	}
	message := &Message{
		Method:    MessageAnnounce,
		Namespace: a.Namespace,
	}
	s.handler.handle(message)
	return nil
}

func (s *Session) onUnannounce(msg *wire.UnannounceMessage) error {
	if !s.incomingAnnouncements.delete(msg.TrackNamespace) {
		return errUnknownAnnouncement
	}
	req := &Message{
		Method:    MessageUnannounce,
		Namespace: msg.TrackNamespace,
	}
	s.handler.handle(req)
	return nil
}

func (s *Session) onAnnounceOk(msg *wire.AnnounceOkMessage) error {
	announcement, err := s.outgoingAnnouncements.confirmAndGet(msg.TrackNamespace)
	if err != nil {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- nil:
	default:
		s.logger.Info("dopping unhandled AnnounceOk response")
	}
	return nil
}

func (s *Session) onAnnounceError(msg *wire.AnnounceErrorMessage) error {
	announcement, ok := s.outgoingAnnouncements.reject(msg.TrackNamespace)
	if !ok {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- ProtocolError{
		code:    msg.ErrorCode,
		message: msg.ReasonPhrase,
	}:
	default:
		s.logger.Info("dropping unhandled AnnounceError response")
	}
	return nil
}

func (s *Session) onSubscribeOk(msg *wire.SubscribeOkMessage) error {
	sub, ok := s.remoteTracks.confirm(msg.SubscribeID)
	if !ok {
		return errUnknownSubscribeID
	}
	select {
	case sub.responseChan <- nil:
	default:
		// TODO: Unsubscribe?
		s.logger.Info("dropping unhandled SubscribeOk response")
	}
	return nil
}

func (s *Session) onSubscribeError(msg *wire.SubscribeErrorMessage) error {
	sub, ok := s.remoteTracks.reject(msg.SubscribeID)
	if !ok {
		return errUnknownSubscribeID
	}
	select {
	case sub.responseChan <- ProtocolError{
		code:    msg.ErrorCode,
		message: msg.ReasonPhrase,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeError response")
	}
	return nil
}

func (s *Session) onSubscribeDone(msg *wire.SubscribeDoneMessage) error {
	sub, ok := s.remoteTracks.findBySubscribeID(msg.SubscribeID)
	if !ok {
		// TODO: Protocol violation?
		return errUnknownSubscribeID
	}
	sub.done(msg.StatusCode, msg.ReasonPhrase)
	// TODO: Remove subscription from outgoingSubscriptions map, but maybe only
	// after timeout to wait for late coming objects?
	return nil
}

func (s *Session) onMaxSubscribeID(msg *wire.MaxSubscribeIDMessage) error {
	return s.remoteTracks.updateMaxSubscribeID(msg.SubscribeID)
}

func (s *Session) onFetchOk(msg *wire.FetchOkMessage) error {
	rt, ok := s.remoteTracks.confirm(msg.SubscribeID)
	if !ok {
		return errUnknownSubscribeID
	}
	select {
	case rt.responseChan <- nil:
	default:
		// TODO: Unsubscribe?
		s.logger.Info("dropping unhandled SubscribeOk response")
	}
	return nil

}

func (s *Session) onFetchError(msg *wire.FetchErrorMessage) error {
	rt, ok := s.remoteTracks.reject(msg.SubscribeID)
	if !ok {
		return errUnknownSubscribeID
	}
	select {
	case rt.responseChan <- ProtocolError{
		code:    msg.ErrorCode,
		message: msg.ReasonPhrase,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeError response")
	}
	return nil
}

func (s *Session) onSubscribeAnnounces(msg *wire.SubscribeAnnouncesMessage) error {
	if err := s.pendingIncomingAnnouncementSubscriptions.add(&announcementSubscription{
		namespace: msg.TrackNamespacePrefix,
	}); err != nil {
		return err
	}
	s.handler.handle(&Message{
		Method:    MessageSubscribeAnnounces,
		Namespace: msg.TrackNamespacePrefix,
	})
	return nil
}

func (s *Session) onSubscribeAnnouncesOk(msg *wire.SubscribeAnnouncesOkMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.delete(msg.TrackNamespacePrefix)
	if !ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe_announces prefix",
		}
	}
	select {
	case as.response <- announcementSubscriptionResponse{
		err: nil,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeAnnounces response")
	}
	return nil
}

func (s *Session) onSubscribeAnnouncesError(msg *wire.SubscribeAnnouncesErrorMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.delete(msg.TrackNamespacePrefix)
	if !ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe_announces prefix",
		}
	}
	select {
	case as.response <- announcementSubscriptionResponse{
		err: ProtocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeAnnounces response")
	}
	return nil
}

func (s *Session) onUnsubscribeAnnounces(msg *wire.UnsubscribeAnnouncesMessage) error {
	s.handler.handle(&Message{
		Method:    MessageUnsubscribeAnnounces,
		Namespace: msg.TrackNamespacePrefix,
	})
	return nil
}

func (s *Session) onFetch(msg *wire.FetchMessage) error {
	m := &Message{
		Method:        MessageFetch,
		Namespace:     msg.TrackNamespace,
		Track:         string(msg.TrackName),
		SubscribeID:   msg.SubscribeID,
		TrackAlias:    0,
		Authorization: "",
		Status:        0,
		LastGroupID:   0,
		LastObjectID:  0,
		NewSessionURI: "",
		ErrorCode:     0,
		ReasonPhrase:  "",
	}
	s.handler.handle(m)
	return nil
}

func (s *Session) onFetchCancel(_ *wire.FetchCancelMessage) error {
	// TODO
	return nil
}

// TODO: Maybe don't immediately close the track and give app a chance to react
// first?
func (s *Session) onUnsubscribe(msg *wire.UnsubscribeMessage) error {
	lt, ok := s.localTracks.delete(msg.SubscribeID)
	if !ok {
		return errUnknownSubscribeID
	}
	lt.unsubscribe()
	return nil
}

func (s *Session) onAnnounceCancel(msg *wire.AnnounceCancelMessage) error {
	s.handler.handle(&Message{
		Method:       MessageAnnounceCancel,
		Namespace:    msg.TrackNamespace,
		ErrorCode:    msg.ErrorCode,
		ReasonPhrase: msg.ReasonPhrase,
	})
	return nil
}

// TODO: Does a track status request expect a response? If so, we need to make
// sure we send one here, in case it is not done by the callback.
func (s *Session) onTrackStatusRequest(msg *wire.TrackStatusRequestMessage) error {
	s.handler.handle(&Message{
		Method:    MessageTrackStatusRequest,
		Namespace: msg.TrackNamespace,
		Track:     msg.TrackName,
	})
	return nil
}

func (s *Session) onTrackStatus(msg *wire.TrackStatusMessage) error {
	s.handler.handle(&Message{
		Method:        MessageTrackStatus,
		Namespace:     msg.TrackNamespace,
		Track:         msg.TrackName,
		Authorization: "",
		Status:        msg.StatusCode,
		LastGroupID:   msg.LastGroupID,
		LastObjectID:  msg.LastObjectID,
		ErrorCode:     0,
		ReasonPhrase:  "",
	})
	return nil
}

func (s *Session) onGoAway(msg *wire.GoAwayMessage) error {
	s.handler.handle(&Message{
		Method:        MessageGoAway,
		NewSessionURI: msg.NewSessionURI,
	})
	return nil
}

func (s *Session) onSubscribeUpdate(_ *wire.SubscribeUpdateMessage) error {
	// TODO
	return nil
}

func (s *Session) addLocalTrack(lt *localTrack) error {
	if lt.subscribeID >= s.maxSubscribeID {
		return errMaxSubscribeIDViolated
	}
	// Update max subscribe ID for peer
	if lt.subscribeID >= s.maxSubscribeID/2 {
		s.maxSubscribeID *= 2
		if err := s.controlMessageSender.queueControlMessage(&wire.MaxSubscribeIDMessage{
			SubscribeID: s.maxSubscribeID,
		}); err != nil {
			s.logger.Error("failed to queue max subscribe ID message", "error", err)
		}
	}
	ok := s.localTracks.addPending(lt)
	if !ok {
		return errDuplicateSubscribeID
	}
	return nil
}

func (s *Session) onSubscribe(msg *wire.SubscribeMessage) error {
	auth, err := validateAuthParameter(msg.Parameters)
	if err != nil {
		return err
	}
	m := &Message{
		Method:        MessageSubscribe,
		SubscribeID:   msg.SubscribeID,
		TrackAlias:    msg.TrackAlias,
		Namespace:     msg.TrackNamespace,
		Track:         string(msg.TrackName),
		Authorization: auth,
		Status:        0,
		LastGroupID:   0,
		LastObjectID:  0,
		NewSessionURI: "",
		ErrorCode:     0,
		ReasonPhrase:  "",
	}
	s.handler.handle(m)
	return nil
}

func (s *Session) onSubscribesBlocked(msg *wire.SubscribesBlockedMessage) error {
	s.logger.Info("received subscribes blocked message", "max_subscribe_id", msg.MaximumSubscribeID)
	return nil
}

func (s *Session) unsubscribe(id uint64) error {
	return s.controlMessageSender.queueControlMessage(&wire.UnsubscribeMessage{
		SubscribeID: id,
	})
}

func (s *Session) fetchCancel(id uint64) error {
	return s.controlMessageSender.queueControlMessage(&wire.FetchCancelMessage{
		SubscribeID: id,
	})
}

func (s *Session) sendClientSetup() error {
	params := map[uint64]wire.Parameter{
		wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
			Type:  wire.MaxSubscribeIDParameterKey,
			Value: s.maxSubscribeID,
		},
	}
	if s.protocol == ProtocolQUIC {
		path := s.path
		params[wire.PathParameterKey] = wire.StringParameter{
			Type:  wire.PathParameterKey,
			Value: path,
		}
	}
	if err := s.controlMessageSender.queueControlMessage(&wire.ClientSetupMessage{
		SupportedVersions: wire.SupportedVersions,
		SetupParameters:   params,
	}); err != nil {
		return err
	}
	return nil
}

type objectMessageParser interface {
	Type() wire.StreamType
	SubscribeID() (uint64, error)
	TrackAlias() (uint64, error)
	Messages() iter.Seq2[*wire.ObjectMessage, error]
}

func (s *Session) handleUniStream(parser objectMessageParser) error {
	var err error
	switch parser.Type() {
	case wire.StreamTypeFetch:
		err = s.readFetchStream(parser)
	case wire.StreamTypeSubgroup:
		err = s.readSubgroupStream(parser)
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) readFetchStream(parser objectMessageParser) error {
	s.logger.Info("reading fetch stream")
	sid, err := parser.SubscribeID()
	if err != nil {
		s.logger.Info("failed to parse subscribe ID", "error", err)
		return err
	}
	rt, ok := s.remoteTrackBySubscribeID(sid)
	if !ok {
		return errUnknownSubscribeID
	}
	return rt.readFetchStream(parser)
}

func (s *Session) readSubgroupStream(parser objectMessageParser) error {
	s.logger.Info("reading subgroup")
	sid, err := parser.TrackAlias()
	if err != nil {
		s.logger.Info("failed to parse subscribe ID", "error", err)
		return err
	}
	rt, ok := s.remoteTrackByTrackAlias(sid)
	if !ok {
		return errUnknownSubscribeID
	}
	return rt.readSubgroupStream(parser)
}

func (s *Session) receiveDatagram(msg *wire.ObjectMessage) error {
	subscription, ok := s.remoteTrackByTrackAlias(msg.TrackAlias)
	if !ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown track alias",
		}
	}
	subscription.push(&Object{
		GroupID:    msg.GroupID,
		SubGroupID: msg.SubgroupID,
		ObjectID:   msg.ObjectID,
		Payload:    msg.ObjectPayload,
	})
	return nil
}

// Local API

func (s *Session) handshakeDone() bool {
	select {
	case <-s.handshakeDoneCh:
		return true
	default:
		return false
	}
}

func (s *Session) waitForHandshakeDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.handshakeDoneCh:
		return nil
	}
}

// Announce announces namespace to the peer. It blocks until a response from the
// peer was received or ctx is cancelled and returns an error if the
// announcement was rejected.
func (s *Session) Announce(ctx context.Context, namespace []string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	a := &announcement{
		Namespace:  namespace,
		parameters: map[uint64]wire.Parameter{},
		response:   make(chan error, 1),
	}
	if err := s.outgoingAnnouncements.add(a); err != nil {
		return err
	}
	am := &wire.AnnounceMessage{
		TrackNamespace: a.Namespace,
		Parameters:     a.parameters,
	}
	if err := s.controlMessageSender.queueControlMessage(am); err != nil {
		_, _ = s.outgoingAnnouncements.reject(a.Namespace)
		return err
	}
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case <-ctx.Done():
		return context.Cause(ctx)
	case res := <-a.response:
		return res
	}
}

func (s *Session) AnnounceCancel(ctx context.Context, namespace []string, errorCode uint64, reason string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	if !s.incomingAnnouncements.delete(namespace) {
		return ErrUnknownAnnouncementNamespace
	}
	acm := &wire.AnnounceCancelMessage{
		TrackNamespace: namespace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reason,
	}
	return s.controlMessageSender.queueControlMessage(acm)
}

// Fetch fetches track in namespace from the peer using id as the subscribe ID.
// It blocks until a response from the peer was received or ctx is cancelled.
func (s *Session) Fetch(
	ctx context.Context,
	namespace []string,
	track string,
) (*RemoteTrack, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	rt := newRemoteTrack()
	id, err := s.remoteTracks.addPending(rt)
	if err != nil {
		var tooManySubscribes errSubscribesBlocked
		if errors.As(err, &tooManySubscribes) {
			previous := s.highestSubscribesBlocked.Swap(tooManySubscribes.maxSubscribeID)
			if previous < tooManySubscribes.maxSubscribeID {
				err = errors.Join(err, s.controlMessageSender.queueControlMessage(&wire.SubscribesBlockedMessage{
					MaximumSubscribeID: tooManySubscribes.maxSubscribeID,
				}))
			}
		}
		return nil, err
	}
	rt.onUnsubscribe(func() error {
		return s.fetchCancel(id)
	})
	cm := &wire.FetchMessage{
		SubscribeID:          id,
		SubscriberPriority:   0,
		GroupOrder:           0,
		FetchType:            wire.FetchTypeStandalone,
		TrackNamespace:       namespace,
		TrackName:            []byte(track),
		StartGroup:           0,
		StartObject:          0,
		EndGroup:             0,
		EndObject:            0,
		JoiningSubscribeID:   0,
		PrecedingGroupOffset: 0,
		Parameters:           map[uint64]wire.Parameter{},
	}
	if err = s.controlMessageSender.queueControlMessage(cm); err != nil {
		_, _ = s.remoteTracks.reject(id)
		return nil, err
	}
	select {
	case <-s.ctx.Done():
		err = context.Cause(s.ctx)
	case <-ctx.Done():
		err = context.Cause(ctx)
	case err = <-rt.responseChan:
	}
	if err != nil {
		s.remoteTracks.reject(id)
		if closeErr := rt.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return rt, nil
}

// Path returns the path of the MoQ session which was exchanged during the
// handshake when using QUIC.
func (s *Session) Path() string {
	return s.path
}

func (s *Session) RequestTrackStatus() {
	// TODO
}

// Subscribe subscribes to track in namespace using id as the subscribe ID. It
// blocks until a response from the peer was received or ctx is cancelled.
func (s *Session) Subscribe(
	ctx context.Context,
	namespace []string,
	name string,
	auth string,
) (*RemoteTrack, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	rt := newRemoteTrack()
	id, alias, err := s.remoteTracks.addPendingWithAlias(rt)
	if err != nil {
		var tooManySubscribes errSubscribesBlocked
		if errors.As(err, &tooManySubscribes) {
			previous := s.highestSubscribesBlocked.Swap(tooManySubscribes.maxSubscribeID)
			if previous < tooManySubscribes.maxSubscribeID {
				err = errors.Join(err, s.controlMessageSender.queueControlMessage(&wire.SubscribesBlockedMessage{
					MaximumSubscribeID: tooManySubscribes.maxSubscribeID,
				}))
			}
		}
		return nil, err
	}
	rt.onUnsubscribe(func() error {
		return s.unsubscribe(id)
	})
	cm := &wire.SubscribeMessage{
		SubscribeID:        id,
		TrackAlias:         alias,
		TrackNamespace:     namespace,
		TrackName:          []byte(name),
		SubscriberPriority: 0,
		GroupOrder:         0,
		FilterType:         0,
		StartGroup:         0,
		StartObject:        0,
		EndGroup:           0,
		Parameters:         map[uint64]wire.Parameter{},
	}
	if len(auth) > 0 {
		cm.Parameters[wire.AuthorizationParameterKey] = &wire.StringParameter{
			Type:  wire.AuthorizationParameterKey,
			Value: auth,
		}
	}
	if err = s.controlMessageSender.queueControlMessage(cm); err != nil {
		_, _ = s.remoteTracks.reject(id)
		return nil, err
	}
	select {
	case <-s.ctx.Done():
		err = context.Cause(s.ctx)
	case <-ctx.Done():
		err = context.Cause(ctx)
	case err = <-rt.responseChan:
	}
	if err != nil {
		s.remoteTracks.reject(id)
		if closeErr := rt.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return rt, nil
}

// SubscribeAnnouncements subscribes to announcements of namespaces with prefix.
// It blocks until a response from the peer is received or ctx is cancelled.
func (s *Session) SubscribeAnnouncements(ctx context.Context, prefix []string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	as := &announcementSubscription{
		namespace: prefix,
		response:  make(chan announcementSubscriptionResponse, 1),
	}
	if err := s.pendingOutgointAnnouncementSubscriptions.add(as); err != nil {
		return err
	}
	sam := &wire.SubscribeAnnouncesMessage{
		TrackNamespacePrefix: as.namespace,
		Parameters:           map[uint64]wire.Parameter{},
	}
	if err := s.controlMessageSender.queueControlMessage(sam); err != nil {
		_, _ = s.pendingOutgointAnnouncementSubscriptions.delete(as.namespace)
		return err
	}
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-as.response:
		return resp.err
	}
}

func (s *Session) UnsubscribeAnnouncements(ctx context.Context, namespace []string) error {
	s.pendingOutgointAnnouncementSubscriptions.delete(namespace)
	uam := &wire.UnsubscribeAnnouncesMessage{
		TrackNamespacePrefix: namespace,
	}
	return s.controlMessageSender.queueControlMessage(uam)
}

// TODO
func (s *Session) acceptAnnouncementSubscription(as []string) error {
	return s.controlMessageSender.queueControlMessage(&wire.SubscribeAnnouncesOkMessage{
		TrackNamespacePrefix: as,
	})
}

// TODO
func (s *Session) rejectAnnouncementSubscription(as []string, c uint64, r string) error {
	return s.controlMessageSender.queueControlMessage(&wire.SubscribeAnnouncesErrorMessage{
		TrackNamespacePrefix: as,
		ErrorCode:            c,
		ReasonPhrase:         r,
	})
}

func (s *Session) Unannounce(ctx context.Context, namespace []string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	if ok := s.outgoingAnnouncements.delete(namespace); ok {
		return ErrUnknownAnnouncementNamespace
	}
	u := &wire.UnannounceMessage{
		TrackNamespace: namespace,
	}
	return s.controlMessageSender.queueControlMessage(u)
}

func (s *Session) acceptFetch(id uint64) error {
	_, ok := s.localTracks.confirm(id)
	if !ok {
		return errUnknownSubscribeID
	}
	return s.controlMessageSender.queueControlMessage(&wire.FetchOkMessage{
		SubscribeID:         id,
		GroupOrder:          1,
		EndOfTrack:          0,
		LargestGroupID:      id,
		LargestObjectID:     id,
		SubscribeParameters: wire.Parameters{},
	})
}

func (s *Session) acceptSubscription(id uint64) error {
	_, ok := s.localTracks.confirm(id)
	if !ok {
		return errUnknownSubscribeID
	}
	return s.controlMessageSender.queueControlMessage(&wire.SubscribeOkMessage{
		SubscribeID:     id,
		Expires:         0,
		GroupOrder:      1,
		ContentExists:   false,
		LargestGroupID:  id,
		LargestObjectID: id,
		Parameters:      wire.Parameters{},
	})
}

func (s *Session) acceptAnnouncement(namespace []string) error {
	if err := s.incomingAnnouncements.confirm(namespace); err != nil {
		return err
	}
	if err := s.controlMessageSender.queueControlMessage(&wire.AnnounceOkMessage{
		TrackNamespace: namespace,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Session) rejectAnnouncement(ns []string, c uint64, r string) error {
	return s.controlMessageSender.queueControlMessage(&wire.AnnounceErrorMessage{
		TrackNamespace: ns,
		ErrorCode:      c,
		ReasonPhrase:   r,
	})
}

func (s *Session) rejectSubscription(id uint64, errorCode uint64, reason string) error {
	lt, ok := s.localTracks.reject(id)
	if !ok {
		return errUnknownSubscribeID
	}
	return s.controlMessageSender.queueControlMessage(&wire.SubscribeErrorMessage{
		SubscribeID:  lt.subscribeID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
		TrackAlias:   lt.trackAlias,
	})
}

func (s *Session) rejectFetch(id uint64, errorCode uint64, reason string) error {
	lt, ok := s.localTracks.reject(id)
	if !ok {
		return errUnknownSubscribeID
	}
	return s.controlMessageSender.queueControlMessage(&wire.FetchErrorMessage{
		SubscribeID:  lt.subscribeID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
	})

}

func (s *Session) subscriptionDone(id, code, count uint64, reason string) error {
	lt, ok := s.localTracks.delete(id)
	if !ok {
		return errUnknownSubscribeID

	}
	return s.controlMessageSender.queueControlMessage(&wire.SubscribeDoneMessage{
		SubscribeID:  lt.subscribeID,
		StatusCode:   code,
		StreamCount:  count,
		ReasonPhrase: reason,
	})
}

func compileMessage(msg wire.ControlMessage) []byte {
	buf := make([]byte, 16, 1500)
	buf = append(buf, msg.Append(buf[16:])...)
	length := len(buf[16:])

	typeLenBuf := quicvarint.Append(buf[:0], uint64(msg.Type()))
	typeLenBuf = quicvarint.Append(typeLenBuf, uint64(length))

	n := copy(buf[0:16], typeLenBuf)
	buf = append(buf[:n], buf[16:]...)

	return buf
}

func validatePathParameter(setupParameters wire.Parameters, protocolIsQUIC bool) (string, error) {
	pathParam, ok := setupParameters[wire.PathParameterKey]
	if !ok {
		if !protocolIsQUIC {
			return "", nil
		}
		return "", ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "missing path parameter",
		}
	}
	if !protocolIsQUIC {
		return "", ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "path parameter is not allowed on non-QUIC connections",
		}
	}
	pathParamValue, ok := pathParam.(wire.StringParameter)
	if !ok {
		return "", ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "invalid path parameter type",
		}
	}
	return pathParamValue.Value, nil
}

func validateMaxSubscribeIDParameter(setupParameters wire.Parameters) (uint64, error) {
	maxSubscribeIDParam, ok := setupParameters[wire.MaxSubscribeIDParameterKey]
	if !ok {
		return 0, nil
	}
	maxSubscribeIDParamValue, ok := maxSubscribeIDParam.(wire.VarintParameter)
	if !ok {
		return 0, ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "invalid max subscribe ID parameter type",
		}
	}
	return maxSubscribeIDParamValue.Value, nil
}

func validateAuthParameter(subscribeParameters wire.Parameters) (string, error) {
	authParam, ok := subscribeParameters[wire.AuthorizationParameterKey]
	if !ok {
		return "", nil
	}
	authParamValue, ok := authParam.(wire.StringParameter)
	if !ok {
		return "", ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "invalid auth parameter type",
		}
	}
	return authParamValue.Value, nil
}
