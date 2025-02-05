package moqtransport

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type sessionCallbacks interface {
	queueControlMessage(wire.ControlMessage) error
	onProtocolViolation(protocolError)
	onMessage(*Message)
}

type session struct {
	logger *slog.Logger

	isServer       bool
	protocolIsQUIC bool
	version        wire.Version
	setupDone      bool

	path       string
	localRole  Role
	remoteRole Role

	callbacks sessionCallbacks

	highestSubscribesBlocked atomic.Uint64

	pendingIncomingAnnouncementSubscriptions *announcementSubscriptionMap
	pendingOutgointAnnouncementSubscriptions *announcementSubscriptionMap

	outgoingSubscriptions *subscriptionMap
	incomingSubscriptions *subscriptionMap

	outgoingAnnouncements *announcementMap
	incomingAnnouncements *announcementMap
}

type sessionOption func(*session) error

func pathParameterOption(path string) sessionOption {
	return func(s *session) error {
		s.path = path
		return nil
	}
}

func roleParameterOption(role Role) sessionOption {
	return func(s *session) error {
		s.localRole = role
		return nil
	}
}

func maxSubscribeIDOption(maxID uint64) sessionOption {
	return func(s *session) error {
		s.incomingSubscriptions = newSubscriptionMap(maxID)
		return nil
	}
}

func newSession(callbacks sessionCallbacks, isServer, isQUIC bool, options ...sessionOption) (*session, error) {
	s := &session{
		logger:                                   defaultLogger,
		isServer:                                 isServer,
		protocolIsQUIC:                           isQUIC,
		version:                                  0,
		setupDone:                                false,
		path:                                     "",
		localRole:                                RolePubSub,
		remoteRole:                               0,
		callbacks:                                callbacks,
		pendingOutgointAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		outgoingSubscriptions:                    newSubscriptionMap(0),
		incomingSubscriptions:                    newSubscriptionMap(100),
		outgoingAnnouncements:                    newAnnouncementMap(),
		incomingAnnouncements:                    newAnnouncementMap(),
	}
	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *session) sendClientSetup() error {
	params := map[uint64]wire.Parameter{
		wire.RoleParameterKey: wire.VarintParameter{
			Type:  wire.RoleParameterKey,
			Value: uint64(s.localRole),
		},
		wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
			Type:  wire.MaxSubscribeIDParameterKey,
			Value: s.incomingSubscriptions.getMaxSubscribeID(),
		},
	}
	if s.protocolIsQUIC {
		params[wire.PathParameterKey] = wire.StringParameter{
			Type:  wire.PathParameterKey,
			Value: s.path,
		}
	}
	if err := s.queueControlMessage(&wire.ClientSetupMessage{
		SupportedVersions: wire.SupportedVersions,
		SetupParameters:   params,
	}); err != nil {
		return err
	}
	return nil
}

func (s *session) queueControlMessage(msg wire.ControlMessage) error {
	return s.callbacks.queueControlMessage(msg)
}

func (s *session) remoteTrackBySubscribeID(id uint64) (*RemoteTrack, bool) {
	sub, ok := s.outgoingSubscriptions.findBySubscribeID(id)
	if ok && sub.remoteTrack != nil {
		return sub.remoteTrack, true
	}
	return nil, false
}

func (s *session) remoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	sub, ok := s.outgoingSubscriptions.findByTrackAlias(alias)
	if ok && sub.remoteTrack != nil {
		return sub.remoteTrack, true
	}
	return nil, false
}

// Local API to trigger outgoing control messages

func (s *session) subscribe(sub *subscription) error {
	if err := s.outgoingSubscriptions.addPending(sub); err != nil {
		var tooManySubscribes errMaxSusbcribeIDViolation
		if errors.As(err, &tooManySubscribes) {
			previous := s.highestSubscribesBlocked.Swap(tooManySubscribes.maxSubscribeID)
			err = protocolError{
				code:    ErrorCodeTooManySubscribes,
				message: fmt.Sprintf("too many subscribes, max_subscrib_id: %v", tooManySubscribes.maxSubscribeID),
			}
			if previous < tooManySubscribes.maxSubscribeID {
				err = errors.Join(err, s.queueControlMessage(&wire.SubscribesBlocked{
					MaximumSubscribeID: tooManySubscribes.maxSubscribeID,
				}))
			}
		}
		return err
	}
	sm := &wire.SubscribeMessage{
		SubscribeID:        sub.ID,
		TrackAlias:         sub.TrackAlias,
		TrackNamespace:     sub.Namespace,
		TrackName:          []byte(sub.Trackname),
		SubscriberPriority: 0,
		GroupOrder:         0,
		FilterType:         0,
		StartGroup:         0,
		StartObject:        0,
		EndGroup:           0,
		EndObject:          0,
		Parameters:         map[uint64]wire.Parameter{},
	}
	if len(sub.Authorization) > 0 {
		sm.Parameters[wire.AuthorizationParameterKey] = &wire.StringParameter{
			Type:  wire.AuthorizationParameterKey,
			Value: sub.Authorization,
		}
	}
	if err := s.queueControlMessage(sm); err != nil {
		_, _ = s.outgoingSubscriptions.reject(sub.ID)
		return err
	}
	return nil
}

// used by RemoteRole
func (s *session) unsubscribe(id uint64) error {
	return s.queueControlMessage(&wire.UnsubscribeMessage{
		SubscribeID: id,
	})
}

func (s *session) announce(
	a *announcement,
) error {
	if err := s.outgoingAnnouncements.add(a); err != nil {
		return err
	}
	am := &wire.AnnounceMessage{
		TrackNamespace: a.Namespace,
		Parameters:     a.parameters,
	}
	if err := s.queueControlMessage(am); err != nil {
		_, _ = s.outgoingAnnouncements.reject(a.Namespace)
		return err
	}
	return nil
}

func (s *session) subscribeAnnounces(as *announcementSubscription) error {
	if err := s.pendingOutgointAnnouncementSubscriptions.add(as); err != nil {
		return err
	}
	sam := &wire.SubscribeAnnouncesMessage{
		TrackNamespacePrefix: []string{},
		Parameters:           map[uint64]wire.Parameter{},
	}
	if err := s.queueControlMessage(sam); err != nil {
		_, _ = s.pendingOutgointAnnouncementSubscriptions.delete(as.namespace)
		return err
	}
	return nil
}

func (s *session) acceptSubscription(id uint64, lt *localTrack) error {
	sub, err := s.incomingSubscriptions.confirm(id)
	if err != nil {
		return err
	}
	sub.localTrack = lt
	if sub.isFetch {
		if err := s.queueControlMessage(&wire.FetchOkMessage{
			SubscribeID:     sub.ID,
			GroupOrder:      sub.GroupOrder,
			LargestGroupID:  0,
			LargestObjectID: 0,
		}); err != nil {
			return err
		}
	} else {
		if err := s.queueControlMessage(&wire.SubscribeOkMessage{
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

func (s *session) rejectSubscription(id uint64, errorCode uint64, reason string) error {
	sub, err := s.incomingSubscriptions.reject(id)
	if err != nil {
		return err
	}
	if sub.isFetch {
		return s.queueControlMessage(&wire.FetchErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    errorCode,
			ReasonPhrase: reason,
		})
	} else {
		return s.queueControlMessage(&wire.SubscribeErrorMessage{
			SubscribeID:  sub.ID,
			ErrorCode:    errorCode,
			ReasonPhrase: reason,
			TrackAlias:   sub.TrackAlias,
		})
	}
}

func (s *session) acceptAnnouncement(namespace []string) error {
	if err := s.incomingAnnouncements.confirm(namespace); err != nil {
		return err
	}
	if err := s.queueControlMessage(&wire.AnnounceOkMessage{
		TrackNamespace: namespace,
	}); err != nil {
		return err
	}
	return nil
}

func (s *session) rejectAnnouncement(namespace []string, errorCode uint64, reason string) error {

	return s.queueControlMessage(&wire.AnnounceErrorMessage{
		TrackNamespace: namespace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reason,
	})
}

func (s *session) acceptAnnouncementSubscription(as announcementSubscription) error {
	return s.queueControlMessage(&wire.SubscribeAnnouncesOkMessage{
		TrackNamespacePrefix: as.namespace,
	})
}

func (s *session) rejectAnnouncementSubscription(as announcementSubscription, code uint64, reason string) error {
	return s.queueControlMessage(&wire.SubscribeAnnouncesErrorMessage{
		TrackNamespacePrefix: as.namespace,
		ErrorCode:            code,
		ReasonPhrase:         reason,
	})

}

// Remote API for handling incoming control messages

func (s *session) onControlMessage(msg wire.ControlMessage) error {
	if s.setupDone {
		switch m := msg.(type) {

		case *wire.SubscribeUpdateMessage:
			return s.onSubscribeUpdate(m)

		case *wire.SubscribeMessage:
			return s.onSubscribe(m)

		case *wire.SubscribeOkMessage:
			return s.onSubscribeOk(m)

		case *wire.SubscribeErrorMessage:
			return s.onSubscribeError(m)

		case *wire.AnnounceMessage:
			return s.onAnnounce(m)

		case *wire.AnnounceOkMessage:
			return s.onAnnounceOk(m)

		case *wire.AnnounceErrorMessage:
			return s.onAnnounceError(m)

		case *wire.UnannounceMessage:
			return s.onUnannounce(m)

		case *wire.UnsubscribeMessage:
			return s.onUnsubscribe(m)

		case *wire.SubscribeDoneMessage:
			return s.onSubscribeDone(m)

		case *wire.AnnounceCancelMessage:
			return s.onAnnounceCancel(m)

		case *wire.TrackStatusRequestMessage:
			return s.onTrackStatusRequest(m)

		case *wire.TrackStatusMessage:
			return s.onTrackStatus(m)

		case *wire.GoAwayMessage:
			return s.onGoAway(m)

		case *wire.SubscribeAnnouncesMessage:
			return s.onSubscribeAnnounces(m)

		case *wire.SubscribeAnnouncesOkMessage:
			return s.onSubscribeAnnouncesOk(m)

		case *wire.SubscribeAnnouncesErrorMessage:
			return s.onSubscribeAnnouncesError(m)

		case *wire.UnsubscribeAnnouncesMessage:
			return s.onUnsubscribeAnnounces(m)

		case *wire.MaxSubscribeIDMessage:
			return s.onMaxSubscribeID(m)

		case *wire.FetchMessage:
			return s.onFetch(m)

		case *wire.FetchCancelMessage:
			return s.onFetchCancel(m)

		case *wire.FetchOkMessage:
			return s.onFetchOk(m)

		case *wire.FetchErrorMessage:
			return s.onFetchError(m)

		case *wire.SubscribesBlocked:
			s.logger.Info("received subscribes blocked message", "max_subscribe_id", m.MaximumSubscribeID)
			return nil

		}

		return protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unexpected message type",
		}
	}

	switch m := msg.(type) {
	case *wire.ClientSetupMessage:
		return s.clientSetup(m)
	case *wire.ServerSetupMessage:
		return s.serverSetup(m)
	}
	return protocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unexpected message type before setup",
	}
}

func (s *session) clientSetup(m *wire.ClientSetupMessage) (err error) {
	if !s.isServer {
		return protocolError{
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
		return protocolError{
			code:    ErrorCodeUnsupportedVersion,
			message: "incompatible versions",
		}
	}
	s.version = wire.Version(selectedVersion)
	s.remoteRole, err = validateRoleParameter(m.SetupParameters)
	if err != nil {
		return err
	}

	s.path, err = validatePathParameter(m.SetupParameters, s.protocolIsQUIC)
	if err != nil {
		return err
	}

	remoteMaxSubscribeID, err := validateMaxSubscribeIDParameter(m.SetupParameters)
	if err != nil {
		return err
	}
	if remoteMaxSubscribeID > 0 {
		if err = s.outgoingSubscriptions.updateMaxSubscribeID(remoteMaxSubscribeID); err != nil {
			return err
		}
	}

	if err := s.queueControlMessage(&wire.ServerSetupMessage{
		SelectedVersion: wire.Version(selectedVersion),
		SetupParameters: map[uint64]wire.Parameter{
			wire.RoleParameterKey: wire.VarintParameter{
				Type:  wire.RoleParameterKey,
				Value: uint64(s.localRole),
			},
			wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
				Type:  wire.MaxSubscribeIDParameterKey,
				Value: 100,
			},
		},
	}); err != nil {
		return err
	}
	s.setupDone = true
	return nil
}

func (s *session) serverSetup(m *wire.ServerSetupMessage) (err error) {
	if s.isServer {
		return protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "server received server setup message",
		}
	}

	if !slices.Contains(wire.SupportedVersions, m.SelectedVersion) {
		return protocolError{
			code:    ErrorCodeUnsupportedVersion,
			message: "incompatible versions",
		}
	}
	s.remoteRole, err = validateRoleParameter(m.SetupParameters)
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

	s.setupDone = true
	return nil
}

func (s *session) onSubscribeUpdate(_ *wire.SubscribeUpdateMessage) error {
	return nil
}

func (s *session) onSubscribe(msg *wire.SubscribeMessage) error {
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
	}
	if err := s.incomingSubscriptions.addPending(sub); err != nil {
		return err
	}
	m := &Message{
		Method:        MessageSubscribe,
		Namespace:     sub.Namespace,
		Track:         sub.Trackname,
		Authorization: sub.Authorization,
		ErrorCode:     0,
		ReasonPhrase:  "",
	}
	s.callbacks.onMessage(m)
	return nil
}

func (s *session) onSubscribeOk(msg *wire.SubscribeOkMessage) error {
	if !s.outgoingSubscriptions.hasPending(msg.SubscribeID) {
		err := protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
		s.callbacks.onProtocolViolation(err)
		return err
	}
	fetch, err := s.outgoingSubscriptions.confirm(msg.SubscribeID)
	if err != nil {
		return err
	}
	fetch.remoteTrack = newRemoteTrack(msg.SubscribeID, s)
	select {
	case fetch.response <- subscriptionResponse{
		err:   nil,
		track: fetch.remoteTrack,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeOk response")
	}
	return nil
}

func (s *session) onSubscribeError(msg *wire.SubscribeErrorMessage) error {
	sub, err := s.outgoingSubscriptions.reject(msg.SubscribeID)
	if err != nil {
		return err
	}
	select {
	case sub.response <- subscriptionResponse{
		err: protocolError{
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

func (s *session) onAnnounce(msg *wire.AnnounceMessage) error {
	if len(msg.TrackNamespace) > 32 {
		return protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "namespace tuple with more than 32 items",
		}
	}
	a := &announcement{
		Namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
	}
	if err := s.incomingAnnouncements.add(a); err != nil {
		pv := &protocolError{}
		if errors.As(err, pv) {
			s.callbacks.onProtocolViolation(*pv)
		}
		return err
	}
	message := &Message{
		Method:    MessageAnnounce,
		Namespace: a.Namespace,
	}
	s.callbacks.onMessage(message)
	return nil
}

func (s *session) onAnnounceOk(msg *wire.AnnounceOkMessage) error {
	announcement, err := s.outgoingAnnouncements.confirmAndGet(msg.TrackNamespace)
	if err != nil {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- announcementResponse{
		err: nil,
	}:
	default:
		s.logger.Info("dopping unhandled AnnouncemeOk response")
	}
	return nil
}

func (s *session) onAnnounceError(msg *wire.AnnounceErrorMessage) error {
	announcement, ok := s.outgoingAnnouncements.reject(msg.TrackNamespace)
	if !ok {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- announcementResponse{
		err: protocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
	}:
	default:
		s.logger.Info("dropping unhandled AnnounceError response")
	}
	return nil
}

func (s *session) onUnannounce(msg *wire.UnannounceMessage) error {
	if !s.incomingAnnouncements.delete(msg.TrackNamespace) {
		s.callbacks.onProtocolViolation(errUnknownAnnouncement)
		return errUnknownAnnouncement
	}
	req := &Message{
		Method:    MessageUnannounce,
		Namespace: msg.TrackNamespace,
	}
	s.callbacks.onMessage(req)
	return nil
}

// TODO: Maybe don't immediately close the track and give app a chance to react
// first?
func (s *session) onUnsubscribe(msg *wire.UnsubscribeMessage) error {
	sub, ok := s.incomingSubscriptions.delete(msg.SubscribeID)
	if !ok {
		s.callbacks.onProtocolViolation(errUnknownSubscribeID)
		return errUnknownSubscribeID
	}
	return sub.localTrack.Close()
}

func (s *session) onSubscribeDone(msg *wire.SubscribeDoneMessage) error {
	sub, ok := s.outgoingSubscriptions.findBySubscribeID(msg.SubscribeID)
	if !ok {
		// TODO: Protocol violation?
		return errUnknownSubscribeID
	}
	sub.remoteTrack.done(msg.StatusCode, msg.ReasonPhrase)
	// TODO: Remove subscription from outgoingSubscriptions map, but maybe only
	// after timeout to wait for late coming objects?
	return nil
}

func (s *session) onAnnounceCancel(msg *wire.AnnounceCancelMessage) error {
	s.callbacks.onMessage(&Message{
		Method:       MessageAnnounceCancel,
		Namespace:    msg.TrackNamespace,
		ErrorCode:    msg.ErrorCode,
		ReasonPhrase: msg.ReasonPhrase,
	})
	return nil
}

// TODO: Does a track status request expect a response? If so, we need to make
// sure we send one here, in case it is not done by the callback.
func (s *session) onTrackStatusRequest(msg *wire.TrackStatusRequestMessage) error {
	s.callbacks.onMessage(&Message{
		Method:    MessageTrackStatusRequest,
		Namespace: msg.TrackNamespace,
		Track:     msg.TrackName,
	})
	return nil
}

func (s *session) onTrackStatus(msg *wire.TrackStatusMessage) error {
	s.callbacks.onMessage(&Message{
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

func (s *session) onGoAway(msg *wire.GoAwayMessage) error {
	s.callbacks.onMessage(&Message{
		Method:        MessageGoAway,
		NewSessionURI: msg.NewSessionURI,
	})
	return nil
}

func (s *session) onSubscribeAnnounces(msg *wire.SubscribeAnnouncesMessage) error {
	if err := s.pendingIncomingAnnouncementSubscriptions.add(&announcementSubscription{
		namespace: msg.TrackNamespacePrefix,
	}); err != nil {
		return err
	}
	s.callbacks.onMessage(&Message{
		Method:    MessageSubscribeAnnounces,
		Namespace: msg.TrackNamespacePrefix,
	})
	return nil
}

func (s *session) onSubscribeAnnouncesOk(msg *wire.SubscribeAnnouncesOkMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.delete(msg.TrackNamespacePrefix)
	if !ok {
		return protocolError{
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

func (s *session) onSubscribeAnnouncesError(msg *wire.SubscribeAnnouncesErrorMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.delete(msg.TrackNamespacePrefix)
	if !ok {
		return protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe_announces prefix",
		}
	}
	select {
	case as.response <- announcementSubscriptionResponse{
		err: protocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeAnnounces response")
	}
	return nil

}

func (s *session) onUnsubscribeAnnounces(msg *wire.UnsubscribeAnnouncesMessage) error {
	s.callbacks.onMessage(&Message{
		Method:    MessageUnsubscribeAnnounces,
		Namespace: msg.TrackNamespacePrefix,
	})
	return nil
}

func (s *session) onMaxSubscribeID(msg *wire.MaxSubscribeIDMessage) error {
	return s.outgoingSubscriptions.updateMaxSubscribeID(msg.SubscribeID)
}

func (s *session) onFetch(msg *wire.FetchMessage) error {
	f := &subscription{
		ID:        msg.SubscribeID,
		Namespace: msg.TrackNamspace,
		Trackname: string(msg.TrackName),
	}
	if err := s.incomingSubscriptions.addPending(f); err != nil {
		return err
	}
	m := &Message{
		Method:    MessageFetch,
		Namespace: f.Namespace,
		Track:     f.Trackname,
	}
	s.callbacks.onMessage(m)
	return nil
}

func (s *session) onFetchCancel(msg *wire.FetchCancelMessage) error {
	return nil
}

func (s *session) onFetchOk(msg *wire.FetchOkMessage) error {
	if !s.outgoingSubscriptions.hasPending(msg.SubscribeID) {
		err := protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
		s.callbacks.onProtocolViolation(err)
		return err
	}
	subscription, err := s.outgoingSubscriptions.confirm(msg.SubscribeID)
	if err != nil {
		return err
	}
	subscription.remoteTrack = newRemoteTrack(msg.SubscribeID, s)
	select {
	case subscription.response <- subscriptionResponse{
		err:   nil,
		track: subscription.remoteTrack,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeOk response")
	}
	return nil
}

func (s *session) onFetchError(msg *wire.FetchErrorMessage) error {
	f, err := s.outgoingSubscriptions.reject(msg.SubscribeID)
	if err != nil {
		return err
	}
	select {
	case f.response <- subscriptionResponse{
		err: protocolError{
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

// Helpers

func validateRoleParameter(setupParameters wire.Parameters) (Role, error) {
	remoteRoleParam, ok := setupParameters[wire.RoleParameterKey]
	if !ok {
		return 0, protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "missing role parameter",
		}
	}
	remoteRoleParamValue, ok := remoteRoleParam.(wire.VarintParameter)
	if !ok {
		return 0, protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "invalid role parameter type",
		}
	}
	switch wire.Role(remoteRoleParamValue.Value) {
	case wire.RolePublisher, wire.RoleSubscriber, wire.RolePubSub:
		return wire.Role(remoteRoleParamValue.Value), nil
	}
	return 0, protocolError{
		code:    ErrorCodeProtocolViolation,
		message: fmt.Sprintf("invalid role parameter value: %v", remoteRoleParamValue.Value),
	}
}

func validatePathParameter(setupParameters wire.Parameters, protocolIsQUIC bool) (string, error) {
	pathParam, ok := setupParameters[wire.PathParameterKey]
	if !ok {
		if !protocolIsQUIC {
			return "", nil
		}
		return "", protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "missing path parameter",
		}
	}
	if !protocolIsQUIC {
		return "", protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "path parameter is not allowed on non-QUIC connections",
		}
	}
	pathParamValue, ok := pathParam.(wire.StringParameter)
	if !ok {
		return "", protocolError{
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
		return 0, protocolError{
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
		return "", protocolError{
			code:    ErrorCodeProtocolViolation,
			message: "invalid auth parameter type",
		}
	}
	return authParamValue.Value, nil
}
