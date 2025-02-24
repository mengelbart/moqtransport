package moqtransport

import (
	"context"
	"errors"
	"fmt"
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

	outgoingAnnouncements *announcementMap
	incomingAnnouncements *announcementMap

	pendingOutgointAnnouncementSubscriptions *announcementSubscriptionMap
	pendingIncomingAnnouncementSubscriptions *announcementSubscriptionMap

	highestSubscribesBlocked atomic.Uint64
	outgoingSubscriptions    *subscriptionMap
	incomingSubscriptions    *subscriptionMap
}

func (s *Session) remoteTrackBySubscribeID(id uint64) (*RemoteTrack, bool) {
	sub, ok := s.outgoingSubscriptions.findBySubscribeID(id)
	rt := sub.getRemoteTrack()
	if !ok || rt == nil {
		return nil, false
	}
	return rt, true
}

func (s *Session) remoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	sub, ok := s.outgoingSubscriptions.findByTrackAlias(alias)
	rt := sub.getRemoteTrack()
	if !ok || rt == nil {
		return nil, false
	}
	return rt, true
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
		case *wire.SubscribesBlocked:
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
	var err error
	s.version = wire.Version(selectedVersion)
	if err != nil {
		return err
	}

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
		if err = s.outgoingSubscriptions.updateMaxSubscribeID(remoteMaxSubscribeID); err != nil {
			return err
		}
	}

	if err := s.controlMessageSender.QueueControlMessage(&wire.ServerSetupMessage{
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
	if err != nil {
		return err
	}

	remoteMaxSubscribeID, err := validateMaxSubscribeIDParameter(m.SetupParameters)
	if err != nil {
		return err
	}
	if err = s.outgoingSubscriptions.updateMaxSubscribeID(remoteMaxSubscribeID); err != nil {
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
	if !s.outgoingSubscriptions.hasPending(msg.SubscribeID) {
		err := ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
		return err
	}
	rt := newRemoteTrack(msg.SubscribeID, s)
	sub, err := s.outgoingSubscriptions.confirm(msg.SubscribeID, rt)
	if err != nil {
		return err
	}
	sub.setRemoteTrack(rt)
	select {
	case sub.response <- subscriptionResponse{
		err:   nil,
		track: rt,
	}:
	default:
		// TODO: Unsubscribe?
		s.logger.Info("dropping unhandled SubscribeOk response")
	}
	return nil
}

func (s *Session) onSubscribeError(msg *wire.SubscribeErrorMessage) error {
	sub, err := s.outgoingSubscriptions.reject(msg.SubscribeID)
	if err != nil {
		return err
	}
	select {
	case sub.response <- subscriptionResponse{
		err: ProtocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
		track: nil,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeError response")
	}
	return nil
}

func (s *Session) onSubscribeDone(msg *wire.SubscribeDoneMessage) error {
	sub, ok := s.outgoingSubscriptions.findBySubscribeID(msg.SubscribeID)
	if !ok {
		// TODO: Protocol violation?
		return errUnknownSubscribeID
	}
	rt := sub.getRemoteTrack()
	if rt != nil {
		rt.done(msg.StatusCode, msg.ReasonPhrase)
	}
	// TODO: Remove subscription from outgoingSubscriptions map, but maybe only
	// after timeout to wait for late coming objects?
	return nil
}

func (s *Session) onMaxSubscribeID(msg *wire.MaxSubscribeIDMessage) error {
	return s.outgoingSubscriptions.updateMaxSubscribeID(msg.SubscribeID)
}

func (s *Session) onFetchOk(msg *wire.FetchOkMessage) error {
	if !s.outgoingSubscriptions.hasPending(msg.SubscribeID) {
		err := ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
		return err
	}
	rt := newRemoteTrack(msg.SubscribeID, s)
	subscription, err := s.outgoingSubscriptions.confirm(msg.SubscribeID, rt)
	if err != nil {
		return err
	}
	select {
	case subscription.response <- subscriptionResponse{
		err:   nil,
		track: rt,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeOk response")
	}
	return nil

}

func (s *Session) onFetchError(msg *wire.FetchErrorMessage) error {
	f, err := s.outgoingSubscriptions.reject(msg.SubscribeID)
	if err != nil {
		return err
	}
	select {
	case f.response <- subscriptionResponse{
		err: ProtocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
		track: nil,
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
	f := &subscription{
		ID:        msg.SubscribeID,
		Namespace: msg.TrackNamspace,
		Trackname: string(msg.TrackName),
		isFetch:   true,
	}
	if err := s.incomingSubscriptions.addPending(f); err != nil {
		return err
	}
	m := &Message{
		Method:    MessageFetch,
		Namespace: f.Namespace,
		Track:     f.Trackname,
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
	sub, ok := s.incomingSubscriptions.delete(msg.SubscribeID)
	if !ok {
		return errUnknownSubscribeID
	}
	sub.localTrack.unsubscribe()
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

func (s *Session) onSubscribe(msg *wire.SubscribeMessage) error {
	auth, err := validateAuthParameter(msg.Parameters)
	if err != nil {
		return err
	}
	sub := &subscription{
		ID:            msg.SubscribeID,
		TrackAlias:    msg.TrackAlias,
		Namespace:     msg.TrackNamespace,
		Trackname:     string(msg.TrackName),
		Authorization: auth,
		isFetch:       false,
	}
	if err := s.incomingSubscriptions.addPending(sub); err != nil {
		var maxSubscribeIDerr errMaxSusbcribeIDViolation
		if errors.As(err, &maxSubscribeIDerr) {
			return ProtocolError{
				code:    ErrorCodeTooManySubscribes,
				message: fmt.Sprintf("too many subscribes, max_subscribe_id: %v", maxSubscribeIDerr.maxSubscribeID),
			}
		}
		return err
	}
	m := &Message{
		Method:        MessageSubscribe,
		SubscribeID:   sub.ID,
		TrackAlias:    sub.TrackAlias,
		Namespace:     sub.Namespace,
		Track:         sub.Trackname,
		Authorization: sub.Authorization,
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

func (s *Session) onSubscribesBlocked(msg *wire.SubscribesBlocked) error {
	s.logger.Info("received subscribes blocked message", "max_subscribe_id", msg.MaximumSubscribeID)
	return nil
}

func (s *Session) unsubscribe(id uint64) error {
	return s.controlMessageSender.QueueControlMessage(&wire.UnsubscribeMessage{
		SubscribeID: id,
	})
}

func (s *Session) sendClientSetup() error {
	params := map[uint64]wire.Parameter{
		wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
			Type:  wire.MaxSubscribeIDParameterKey,
			Value: s.incomingSubscriptions.getMaxSubscribeID(),
		},
	}
	if s.protocol == ProtocolQUIC {
		path := s.path
		params[wire.PathParameterKey] = wire.StringParameter{
			Type:  wire.PathParameterKey,
			Value: path,
		}
	}
	if err := s.controlMessageSender.QueueControlMessage(&wire.ClientSetupMessage{
		SupportedVersions: wire.SupportedVersions,
		SetupParameters:   params,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Session) handleUniStream(stream ReceiveStream) error {
	s.logger.Info("handling new uni stream")
	parser, err := wire.NewObjectStreamParser(stream)
	if err != nil {
		return err
	}
	s.logger.Info("parsed object stream header")
	switch parser.Typ {
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

func (s *Session) readFetchStream(parser *wire.ObjectStreamParser) error {
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

func (s *Session) readSubgroupStream(parser *wire.ObjectStreamParser) error {
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

func (s *Session) receiveDatagram(dgram []byte) error {
	msg := new(wire.ObjectMessage)
	_, err := msg.ParseDatagram(dgram)
	if err != nil {
		s.logger.Error("failed to parse datagram object", "error", err)
		return err
	}
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
	if err := s.controlMessageSender.QueueControlMessage(am); err != nil {
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
	return s.controlMessageSender.QueueControlMessage(acm)
}

// Fetch fetches track in namespace from the peer using id as the subscribe ID.
// It blocks until a response from the peer was received or ctx is cancelled.
func (s *Session) Fetch(
	ctx context.Context,
	id uint64,
	namespace []string,
	track string,
) (*RemoteTrack, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	f := &subscription{
		ID:        id,
		Namespace: namespace,
		Trackname: track,
		isFetch:   true,
		response:  make(chan subscriptionResponse, 1),
	}
	if err := s.outgoingSubscriptions.addPending(f); err != nil {
		var tooManySubscribes errMaxSusbcribeIDViolation
		if errors.As(err, &tooManySubscribes) {
			previous := s.highestSubscribesBlocked.Swap(tooManySubscribes.maxSubscribeID)
			if previous < tooManySubscribes.maxSubscribeID {
				err = errors.Join(err, s.controlMessageSender.QueueControlMessage(&wire.SubscribesBlocked{
					MaximumSubscribeID: tooManySubscribes.maxSubscribeID,
				}))
			}
		}
		return nil, err
	}
	cm := &wire.FetchMessage{
		SubscribeID:          f.ID,
		SubscriberPriority:   0,
		GroupOrder:           0,
		FetchType:            wire.FetchTypeStandalone,
		TrackNamspace:        f.Namespace,
		TrackName:            []byte(f.Trackname),
		StartGroup:           0,
		StartObject:          0,
		EndGroup:             0,
		EndObject:            0,
		JoiningSubscribeID:   id,
		PrecedingGroupOffset: 0,
		Parameters:           map[uint64]wire.Parameter{},
	}
	if err := s.controlMessageSender.QueueControlMessage(cm); err != nil {
		_, _ = s.outgoingSubscriptions.reject(f.ID)
		return nil, err
	}
	return s.subscribe(ctx, f)
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
	id, alias uint64,
	namespace []string,
	name string,
	auth string,
) (*RemoteTrack, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	ps := &subscription{
		ID:            id,
		TrackAlias:    alias,
		Namespace:     namespace,
		Trackname:     name,
		Authorization: auth,
		Expires:       0,
		GroupOrder:    0,
		ContentExists: false,
		response:      make(chan subscriptionResponse, 1),
	}
	if err := s.outgoingSubscriptions.addPending(ps); err != nil {
		var tooManySubscribes errMaxSusbcribeIDViolation
		if errors.As(err, &tooManySubscribes) {
			previous := s.highestSubscribesBlocked.Swap(tooManySubscribes.maxSubscribeID)
			if previous < tooManySubscribes.maxSubscribeID {
				err = errors.Join(err, s.controlMessageSender.QueueControlMessage(&wire.SubscribesBlocked{
					MaximumSubscribeID: tooManySubscribes.maxSubscribeID,
				}))
			}
		}
		return nil, err
	}
	cm := &wire.SubscribeMessage{
		SubscribeID:        ps.ID,
		TrackAlias:         ps.TrackAlias,
		TrackNamespace:     ps.Namespace,
		TrackName:          []byte(ps.Trackname),
		SubscriberPriority: 0,
		GroupOrder:         0,
		FilterType:         0,
		StartGroup:         0,
		StartObject:        0,
		EndGroup:           0,
		Parameters:         map[uint64]wire.Parameter{},
	}
	if len(ps.Authorization) > 0 {
		cm.Parameters[wire.AuthorizationParameterKey] = &wire.StringParameter{
			Type:  wire.AuthorizationParameterKey,
			Value: ps.Authorization,
		}
	}
	if err := s.controlMessageSender.QueueControlMessage(cm); err != nil {
		_, _ = s.outgoingSubscriptions.reject(ps.ID)
		return nil, err
	}
	return s.subscribe(ctx, ps)
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
	if err := s.controlMessageSender.QueueControlMessage(sam); err != nil {
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
	return s.controlMessageSender.QueueControlMessage(uam)
}

// TODO
func (s *Session) acceptAnnouncementSubscription(as []string) error {
	return s.controlMessageSender.QueueControlMessage(&wire.SubscribeAnnouncesOkMessage{
		TrackNamespacePrefix: as,
	})
}

// TODO
func (s *Session) rejectAnnouncementSubscription(as []string, c uint64, r string) error {
	return s.controlMessageSender.QueueControlMessage(&wire.SubscribeAnnouncesErrorMessage{
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
	return s.controlMessageSender.QueueControlMessage(u)
}

func (s *Session) subscribe(
	ctx context.Context,
	ps *subscription,
) (*RemoteTrack, error) {
	select {
	case <-s.ctx.Done():
		return nil, context.Cause(s.ctx)
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ps.response:
		return res.track, res.err
	}
}

func (s *Session) acceptSubscription(id uint64, lt *localTrack) error {
	sub, err := s.incomingSubscriptions.confirm(id, nil)
	if err != nil {
		return err
	}
	sub.localTrack = lt
	if sub.GroupOrder == 0 {
		sub.GroupOrder = 0x1
	}
	if sub.isFetch {
		if err := s.controlMessageSender.QueueControlMessage(&wire.FetchOkMessage{
			SubscribeID:     sub.ID,
			GroupOrder:      sub.GroupOrder,
			LargestGroupID:  0,
			LargestObjectID: 0,
		}); err != nil {
			return err
		}
	} else {
		if err := s.controlMessageSender.QueueControlMessage(&wire.SubscribeOkMessage{
			SubscribeID:     sub.ID,
			Expires:         sub.Expires,
			GroupOrder:      sub.GroupOrder,
			ContentExists:   sub.ContentExists,
			LargestGroupID:  0,
			LargestObjectID: 0,
			Parameters:      map[uint64]wire.Parameter{},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) acceptAnnouncement(namespace []string) error {
	if err := s.incomingAnnouncements.confirm(namespace); err != nil {
		return err
	}
	if err := s.controlMessageSender.QueueControlMessage(&wire.AnnounceOkMessage{
		TrackNamespace: namespace,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Session) rejectAnnouncement(ns []string, c uint64, r string) error {
	return s.controlMessageSender.QueueControlMessage(&wire.AnnounceErrorMessage{
		TrackNamespace: ns,
		ErrorCode:      c,
		ReasonPhrase:   r,
	})
}

func (s *Session) rejectSubscription(id uint64, errorCode uint64, reason string) error {
	sub, err := s.incomingSubscriptions.reject(id)
	if err != nil {
		return err
	}
	if sub.isFetch {
		return s.controlMessageSender.QueueControlMessage(&wire.FetchErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    errorCode,
			ReasonPhrase: reason,
		})
	} else {
		return s.controlMessageSender.QueueControlMessage(&wire.SubscribeErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    errorCode,
			ReasonPhrase: reason,
			TrackAlias:   sub.TrackAlias,
		})
	}
}

func (s *Session) subscriptionDone(id, code, count uint64, reason string) error {
	sub, ok := s.incomingSubscriptions.delete(id)
	if !ok {
		return errUnknownSubscribeID

	}
	return s.controlMessageSender.QueueControlMessage(&wire.SubscribeDoneMessage{
		SubscribeID:  sub.ID,
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
