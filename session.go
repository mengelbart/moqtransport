package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/mengelbart/moqtransport/internal/container"
	"github.com/mengelbart/moqtransport/internal/wire"
)

var (
	errDuplicateSubscribeID    = errors.New("duplicate subscribe ID")
	errMaxSubscribeIDViolation = errors.New("selected subscribe ID violates remote maximum subscribe ID")
	errUnknownSubscribeID      = errors.New("unknown subscribe ID")
)

type sessionCallbacks interface {
	queueControlMessage(wire.ControlMessage) error
	onSubscription(Subscription) bool
	onAnnouncement(Announcement) bool
	onAnnouncementSubscription(AnnouncementSubscription) bool
}

type session struct {
	lock sync.Mutex

	logger *slog.Logger

	isServer       bool
	protocolIsQUIC bool
	version        wire.Version
	setupDone      bool

	path       string
	localRole  Role
	remoteRole Role

	callbacks sessionCallbacks

	pendingOutgointAnnouncementSubscriptions []AnnouncementSubscription
	outgointAnnouncementSubscriptions        []AnnouncementSubscription

	remoteMaxSubscribeID         uint64
	pendingOutgoingSubscriptions map[uint64]Subscription
	outgoingSubscriptions        map[uint64]Subscription
	trackAliasToSusbcribeID      map[uint64]uint64

	localMaxSubscribeID   uint64
	incomingSubscriptions map[uint64]Subscription

	pendingOutgoingAnnouncements []Announcement
	outgoingAnnouncements        []Announcement

	incomingAnnouncements *container.Trie[string, Announcement]
}

type sessionOption func(*session)

func pathParameterOption(path string) sessionOption {
	return func(s *session) {
		s.path = path
	}
}

func roleParameterOption(role Role) sessionOption {
	return func(s *session) {
		s.localRole = role
	}
}

func maxSubscribeIDOption(maxID uint64) sessionOption {
	return func(s *session) {
		s.localMaxSubscribeID = maxID
	}
}

func newSession(callbacks sessionCallbacks, isServer, isQUIC bool, options ...sessionOption) (*session, error) {
	s := &session{
		lock: sync.Mutex{},

		logger: defaultLogger,

		isServer:       isServer,
		protocolIsQUIC: isQUIC,
		version:        0,
		setupDone:      false,

		path:       "",
		localRole:  RolePubSub,
		remoteRole: 0,

		callbacks: callbacks,

		remoteMaxSubscribeID:         0,
		pendingOutgoingSubscriptions: map[uint64]Subscription{},
		outgoingSubscriptions:        map[uint64]Subscription{},
		trackAliasToSusbcribeID:      map[uint64]uint64{},

		localMaxSubscribeID:   100,
		incomingSubscriptions: map[uint64]Subscription{},

		pendingOutgoingAnnouncements: []Announcement{},
		outgoingAnnouncements:        []Announcement{},

		incomingAnnouncements: container.NewTrie[string, Announcement](),
	}
	for _, opt := range options {
		opt(s)
	}
	if !s.isServer {
		params := map[uint64]wire.Parameter{
			wire.RoleParameterKey: wire.VarintParameter{
				Type:  wire.RoleParameterKey,
				Value: uint64(s.localRole),
			},
			wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
				Type:  wire.MaxSubscribeIDParameterKey,
				Value: s.localMaxSubscribeID,
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
			return nil, err
		}
	}
	return s, nil
}

func (s *session) queueControlMessage(msg wire.ControlMessage) error {
	return s.callbacks.queueControlMessage(msg)
}

func (s *session) remoteTrackBySubscribeID(id uint64) (*RemoteTrack, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	sub, ok := s.outgoingSubscriptions[id]
	if !ok {
		s.logger.Info("could not find remote track for subscribe ID", "subscribeID", id)
		return nil, false
	}
	return sub.remoteTrack, true
}

func (s *session) subscribeIDForTrackAlias(alias uint64) (uint64, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	id, ok := s.trackAliasToSusbcribeID[alias]
	if !ok {
		s.logger.Info("could not find remote track for track alias", "trackAlias", alias)
		return 0, false
	}
	return id, true
}

func (s *session) remoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	id, ok := s.subscribeIDForTrackAlias(alias)
	if !ok {
		return nil, false
	}
	return s.remoteTrackBySubscribeID(id)
}

// Local API to trigger outgoing control messages

func (s *session) subscribe(
	ctx context.Context,
	sub Subscription,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if sub.ID >= s.remoteMaxSubscribeID {
		return errMaxSubscribeIDViolation
	}
	if _, ok := s.pendingOutgoingSubscriptions[sub.ID]; ok {
		return errDuplicateSubscribeID
	}
	if _, ok := s.outgoingSubscriptions[sub.ID]; ok {
		return errDuplicateSubscribeID
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
		return err
	}
	s.pendingOutgoingSubscriptions[sub.ID] = sub
	return nil
}

func (s *session) announce(
	namespace []string,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	am := &wire.AnnounceMessage{
		TrackNamespace: namespace,
		Parameters:     map[uint64]wire.Parameter{},
	}
	if err := s.queueControlMessage(am); err != nil {
		return err
	}
	s.pendingOutgoingAnnouncements = append(s.pendingOutgoingAnnouncements, Announcement{
		Namespace:  namespace,
		parameters: map[uint64]wire.Parameter{},
	})
	return nil
}

func (s *session) subscribeAnnounces(subscription AnnouncementSubscription) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	sam := &wire.SubscribeAnnouncesMessage{
		TrackNamespacePrefix: []string{},
		Parameters:           map[uint64]wire.Parameter{},
	}
	if err := s.queueControlMessage(sam); err != nil {
		return err
	}
	s.pendingOutgointAnnouncementSubscriptions = append(s.pendingOutgointAnnouncementSubscriptions, AnnouncementSubscription{
		namespace: subscription.namespace,
	})
	return nil
}

func (s *session) acceptSubscription(sub Subscription) error {
	s.lock.Lock()
	defer s.lock.Unlock()

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
	s.incomingSubscriptions[sub.ID] = sub
	return nil
}

func (s *session) rejectSubscription(sub Subscription, errorCode uint64, reason string) error {
	return s.queueControlMessage(&wire.SubscribeErrorMessage{
		SubscribeID:  sub.ID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
		TrackAlias:   sub.TrackAlias,
	})
}

func (s *session) acceptAnnouncement(a Announcement) error {
	if err := s.queueControlMessage(&wire.AnnounceOkMessage{
		TrackNamespace: a.Namespace,
	}); err != nil {
		return err
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.incomingAnnouncements.Put(a.Namespace, a)
	return nil
}

func (s *session) rejectAnnouncement(a Announcement, errorCode uint64, reason string) error {
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

	s.remoteMaxSubscribeID, err = validateMaxSubscribeIDParameter(m.SetupParameters)
	if err != nil {
		return err
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

	s.remoteMaxSubscribeID, err = validateMaxSubscribeIDParameter(m.SetupParameters)
	if err != nil {
		return err
	}
	s.setupDone = true
	return nil
}

func (s *session) onSubscribeUpdate(msg *wire.SubscribeUpdateMessage) error {
	return nil
}

func (s *session) onSubscribe(msg *wire.SubscribeMessage) error {
	s.lock.Lock()
	auth, err := validateAuthParameter(msg.Parameters)
	if err != nil {
		return err
	}
	if msg.SubscribeID > s.localMaxSubscribeID {
		return ProtocolError{
			code:    ErrorTooManySubscribes,
			message: "",
		}
	}
	if _, ok := s.incomingSubscriptions[msg.SubscribeID]; ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "duplicate subscribe ID",
		}
	}
	sub := Subscription{
		ID:            msg.SubscribeID,
		TrackAlias:    msg.TrackAlias,
		Namespace:     msg.TrackNamespace,
		Trackname:     string(msg.TrackName),
		Authorization: auth,
	}
	s.lock.Unlock()

	if !s.callbacks.onSubscription(sub) {
		return s.rejectSubscription(sub, SubscribeErrorUnhandled, "unhandled subscription")
	}
	return nil
}

func (s *session) onSubscribeOk(msg *wire.SubscribeOkMessage) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	subscription, ok := s.pendingOutgoingSubscriptions[msg.SubscribeID]
	if !ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
	}
	s.logger.Info("got subscribeOk", "subscription", subscription)
	delete(s.pendingOutgoingSubscriptions, msg.SubscribeID)
	subscription.remoteTrack = newRemoteTrack(subscription.ID)
	s.logger.Info("got subscribeOk", "subscription", subscription, "track", subscription.remoteTrack)
	s.outgoingSubscriptions[msg.SubscribeID] = subscription
	s.trackAliasToSusbcribeID[subscription.TrackAlias] = subscription.ID
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
	s.lock.Lock()
	defer s.lock.Unlock()

	sub, ok := s.pendingOutgoingSubscriptions[msg.SubscribeID]
	if !ok {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
	}
	delete(s.pendingOutgoingSubscriptions, msg.SubscribeID)
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
	s.lock.Lock()
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
	s.lock.Unlock()

	a := Announcement{
		Namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
	}
	if !s.callbacks.onAnnouncement(a) {
		return s.rejectAnnouncement(a, AnnouncementUnhandled, "unhandled announcement")
	}
	return nil
}

func (s *session) onAnnounceOk(msg *wire.AnnounceOkMessage) error {
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
	s.lock.Lock()
	defer s.lock.Unlock()

	i := slices.IndexFunc(s.pendingOutgointAnnouncementSubscriptions, func(as AnnouncementSubscription) bool {
		return slices.Equal(as.namespace, msg.TrackNamespacePrefix)
	})
	if i == -1 {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe_announces prefix",
		}
	}
	s.logger.Info("got subscribeAnnouncesOk", "prefix", msg.TrackNamespacePrefix)
	as := s.pendingOutgointAnnouncementSubscriptions[i]
	s.outgointAnnouncementSubscriptions = append(s.outgointAnnouncementSubscriptions, as)
	s.pendingOutgointAnnouncementSubscriptions = append(s.pendingOutgointAnnouncementSubscriptions[:i], s.pendingOutgointAnnouncementSubscriptions[i+1:]...)
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
	s.lock.Lock()
	defer s.lock.Unlock()

	if msg.SubscribeID <= s.remoteMaxSubscribeID {
		return ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "max subscribe ID decreased",
		}
	}
	s.remoteMaxSubscribeID = msg.SubscribeID
	return nil
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
