package moqtransport

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/mengelbart/moqtransport/internal/container"
	"github.com/mengelbart/moqtransport/internal/wire"
)

var errUnknownAnnouncement = errors.New("unknown announcement")

type sessionCallbacks interface {
	queueControlMessage(wire.ControlMessage) error
	onProtocolViolation(ProtocolError)
	onRequest(*Request)
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

	pendingOutgointAnnouncementSubscriptions *announcementSubscriptionMap

	outgoingSubscriptions *subscriptionMap
	incomingSubscriptions *subscriptionMap

	outgoingAnnouncements *announcementMap

	incomingAnnouncements *container.Trie[string, *Announcement]
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
		incomingAnnouncements:                    container.NewTrie[string, *Announcement](),
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
	return s.outgoingSubscriptions.remoteTrackBySubscribeID(id)
}

func (s *session) remoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	return s.outgoingSubscriptions.remoteTrackByTrackAlias(alias)
}

// Local API to trigger outgoing control messages

func (s *session) subscribe(sub *Subscription) error {
	if err := s.outgoingSubscriptions.addPending(sub); err != nil {
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

func (s *session) announce(
	a Announcement,
) error {
	if err := s.outgoingAnnouncements.add(a); err != nil {
		return err
	}
	am := &wire.AnnounceMessage{
		TrackNamespace: a.Namespace,
		Parameters:     a.parameters,
	}
	if err := s.queueControlMessage(am); err != nil {
		_, _ = s.outgoingAnnouncements.delete(a.Namespace)
		return err
	}
	return nil
}

func (s *session) subscribeAnnounces(subscription AnnouncementSubscription) error {
	if err := s.pendingOutgointAnnouncementSubscriptions.add(AnnouncementSubscription{
		namespace: subscription.namespace,
	}); err != nil {
		return err
	}
	sam := &wire.SubscribeAnnouncesMessage{
		TrackNamespacePrefix: []string{},
		Parameters:           map[uint64]wire.Parameter{},
	}
	if err := s.queueControlMessage(sam); err != nil {
		_, _ = s.pendingOutgointAnnouncementSubscriptions.delete(subscription.namespace)
		return err
	}
	return nil
}

func (s *session) acceptSubscription(sub *Subscription) error {
	err := s.incomingSubscriptions.confirm(sub)
	if err != nil {
		return err
	}
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
	return nil
}

func (s *session) rejectSubscription(sub *Subscription, errorCode uint64, reason string) error {
	return s.queueControlMessage(&wire.SubscribeErrorMessage{
		SubscribeID:  sub.ID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
		TrackAlias:   sub.TrackAlias,
	})
}

func (s *session) acceptAnnouncement(a *Announcement) error {
	if err := s.queueControlMessage(&wire.AnnounceOkMessage{
		TrackNamespace: a.Namespace,
	}); err != nil {
		return err
	}
	s.incomingAnnouncements.Put(a.Namespace, a)
	return nil
}

func (s *session) rejectAnnouncement(a *Announcement, errorCode uint64, reason string) error {
	return s.queueControlMessage(&wire.AnnounceErrorMessage{
		TrackNamespace: a.Namespace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reason,
	})
}

func (s *session) acceptAnnouncementSubscription(as AnnouncementSubscription) error {
	return s.queueControlMessage(&wire.SubscribeAnnouncesOkMessage{
		TrackNamespacePrefix: as.namespace,
	})
}

func (s *session) rejectAnnouncementSubscription(as AnnouncementSubscription, code uint64, reason string) error {
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
		}

		return ProtocolError{
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
	return ProtocolError{
		code:    ErrorCodeProtocolViolation,
		message: "unexpected message type before setup",
	}
}

func (s *session) clientSetup(m *wire.ClientSetupMessage) (err error) {
	if !s.isServer {
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
	sub := &Subscription{
		ID:            msg.SubscribeID,
		TrackAlias:    msg.TrackAlias,
		Namespace:     msg.TrackNamespace,
		Trackname:     string(msg.TrackName),
		Authorization: auth,
	}
	if err := s.incomingSubscriptions.addPending(sub); err != nil {
		pv := &ProtocolError{}
		if errors.As(err, pv) {
			s.callbacks.onProtocolViolation(*pv)
		}
		return err
	}
	req := &Request{
		Method:       MethodSubscribe,
		Subscription: sub,
	}
	s.callbacks.onRequest(req)
	return nil
}

func (s *session) onSubscribeOk(msg *wire.SubscribeOkMessage) error {
	subscription, err := s.outgoingSubscriptions.confirmAndGet(msg.SubscribeID)
	if err != nil {
		return err
	}
	select {
	case subscription.response <- subscriptionResponse{
		err:   nil,
		track: subscription.remoteTrack,
	}:
	default:
		panic("TODO unhandled subscription response")
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
		err: ProtocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
		track: nil,
	}:
	default:
		panic("TODO unhandled subscription response")
	}
	return nil
}

func (s *session) onAnnounce(msg *wire.AnnounceMessage) error {
	if len(msg.TrackNamespace) > 32 {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "namespace tuple with more than 32 items",
		}
	}
	if _, ok := s.incomingAnnouncements.Get(msg.TrackNamespace); ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "duplicate announcement",
		}
	}
	if _, ok := s.incomingAnnouncements.Get(msg.TrackNamespace); ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "duplicate announcement",
		}
	}
	a := &Announcement{
		Namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
	}
	req := &Request{
		Method:       MethodAnnounce,
		Announcement: a,
	}
	s.callbacks.onRequest(req)
	return nil
}

func (s *session) onAnnounceOk(msg *wire.AnnounceOkMessage) error {
	announcement, ok := s.outgoingAnnouncements.delete(msg.TrackNamespace)
	if !ok {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- announcementResponse{
		err: nil,
	}:
	default:
		panic("TODO unhandled announcement response")
	}
	return nil
}

func (s *session) onAnnounceError(msg *wire.AnnounceErrorMessage) error {
	return nil
}

func (s *session) onUnannounce(msg *wire.UnannounceMessage) error {
	return nil
}

func (s *session) onUnsubscribe(msg *wire.UnsubscribeMessage) error {
	return nil
}

func (s *session) onSubscribeDone(msg *wire.SubscribeDoneMessage) error {
	return nil
}

func (s *session) onAnnounceCancel(msg *wire.AnnounceCancelMessage) error {
	return nil
}

func (s *session) onTrackStatusRequest(msg *wire.TrackStatusRequestMessage) error {
	return nil
}

func (s *session) onTrackStatus(msg *wire.TrackStatusMessage) error {
	return nil
}

func (s *session) onGoAway(msg *wire.GoAwayMessage) error {
	return nil
}

func (s *session) onSubscribeAnnounces(msg *wire.SubscribeAnnouncesMessage) error {
	return nil
}

func (s *session) onSubscribeAnnouncesOk(msg *wire.SubscribeAnnouncesOkMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.delete(msg.TrackNamespacePrefix)
	if !ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe_announces prefix",
		}
	}
	s.logger.Info("got subscribeAnnouncesOk", "prefix", msg.TrackNamespacePrefix)
	select {
	case as.response <- announcementSubscriptionResponse{
		err: nil,
	}:
	default:
		panic("TODO: unhandled announcement subscription response")
	}
	return nil
}

func (s *session) onSubscribeAnnouncesError(msg *wire.SubscribeAnnouncesErrorMessage) error {
	return nil
}

func (s *session) onUnsubscribeAnnounces(msg *wire.UnsubscribeAnnouncesMessage) error {
	return nil
}

func (s *session) onMaxSubscribeID(msg *wire.MaxSubscribeIDMessage) error {
	return s.outgoingSubscriptions.updateMaxSubscribeID(msg.SubscribeID)
}

func (s *session) onFetch(msg *wire.FetchMessage) error {
	return nil
}

func (s *session) onFetchCancel(msg *wire.FetchCancelMessage) error {
	return nil
}

func (s *session) onFetchOk(msg *wire.FetchOkMessage) error {
	return nil
}

func (s *session) onFetchError(msg *wire.FetchErrorMessage) error {
	return nil
}

// Helpers

func validateRoleParameter(setupParameters wire.Parameters) (Role, error) {
	remoteRoleParam, ok := setupParameters[wire.RoleParameterKey]
	if !ok {
		return 0, ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "missing role parameter",
		}
	}
	remoteRoleParamValue, ok := remoteRoleParam.(wire.VarintParameter)
	if !ok {
		return 0, ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "invalid role parameter type",
		}
	}
	switch wire.Role(remoteRoleParamValue.Value) {
	case wire.RolePublisher, wire.RoleSubscriber, wire.RolePubSub:
		return wire.Role(remoteRoleParamValue.Value), nil
	}
	return 0, ProtocolError{
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
