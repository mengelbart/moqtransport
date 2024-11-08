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

const (
	controlMessageQueueSize                   = 1024
	pendingRemoteSubscriptionQueueSize        = 1024
	pendingLocalSubscriptionResponseQueueSize = 1024
	pendingLocalAnnouncementsQueueSize        = 1024
	pendingRemoteAnnouncementsQueueSize       = 1024
)

var (
	errControlMessageQueueOverflow = errors.New("control message if full, message not queued")
	errDuplicateSubscribeID        = errors.New("duplicate subscribe ID")
	errMaxSubscribeIDViolation     = errors.New("selected subscribe ID violates remote maximum subscribe ID")
	errUnknownSubscribeID          = errors.New("unknown subscribe ID")
	errUnknownNamespace            = errors.New("unknown namespace")
)

type Session struct {
	lock sync.Mutex

	logger *slog.Logger

	isServer       bool
	protocolIsQUIC bool
	version        wire.Version
	setupDone      bool

	path       string
	localRole  Role
	remoteRole Role

	controlMessageQueue chan wire.ControlMessage

	remoteMaxSubscribeID         uint64
	pendingOutgoingSubscriptions map[uint64]Subscription
	outgoingSubscriptions        map[uint64]Subscription
	trackAliasToSusbcribeID      map[uint64]uint64

	localMaxSubscribeID       uint64
	incomingSubscriptionQueue chan Subscription
	incomingSubscriptions     map[uint64]Subscription

	pendingOutgoingAnnouncements *container.Trie[Announcement]
	outgoingAnnouncements        *container.Trie[Announcement]

	incomingAnnouncementQueue chan Announcement
	incomingAnnouncements     *container.Trie[Announcement]
}

type SessionOption func(*Session)

func PathParameterOption(path string) SessionOption {
	return func(s *Session) {
		s.path = path
	}
}

func RoleParameterOption(role Role) SessionOption {
	return func(s *Session) {
		s.localRole = role
	}
}

func MaxSubscribeIDOption(maxID uint64) SessionOption {
	return func(s *Session) {
		s.localMaxSubscribeID = maxID
	}
}

func NewSession(isServer, isQUIC bool, options ...SessionOption) (*Session, error) {
	s := &Session{
		lock: sync.Mutex{},

		logger: defaultLogger,

		isServer:       isServer,
		protocolIsQUIC: isQUIC,
		version:        0,
		setupDone:      false,

		path:       "",
		localRole:  RolePubSub,
		remoteRole: 0,

		controlMessageQueue: make(chan wire.ControlMessage, controlMessageQueueSize),

		remoteMaxSubscribeID:         0,
		pendingOutgoingSubscriptions: map[uint64]Subscription{},
		outgoingSubscriptions:        map[uint64]Subscription{},
		trackAliasToSusbcribeID:      map[uint64]uint64{},

		localMaxSubscribeID:       0,
		incomingSubscriptionQueue: make(chan Subscription, pendingRemoteSubscriptionQueueSize),
		incomingSubscriptions:     map[uint64]Subscription{},

		pendingOutgoingAnnouncements: container.NewTrie[Announcement](),
		outgoingAnnouncements:        container.NewTrie[Announcement](),

		incomingAnnouncementQueue: make(chan Announcement, pendingRemoteAnnouncementsQueueSize),
		incomingAnnouncements:     container.NewTrie[Announcement](),
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

func (s *Session) queueControlMessage(msg wire.ControlMessage) error {
	select {
	case s.controlMessageQueue <- msg:
		return nil
	default:
		return errControlMessageQueueOverflow
	}
}

func (s *Session) SendControlMessages() <-chan wire.ControlMessage {
	return s.controlMessageQueue
}

func (s *Session) IncomingSubscriptions() <-chan Subscription {
	return s.incomingSubscriptionQueue
}

func (s *Session) IncomingAnnouncements() <-chan Announcement {
	return s.incomingAnnouncementQueue
}

func (s *Session) RemoteTrackBySubscribeID(id uint64) (*RemoteTrack, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	sub, ok := s.outgoingSubscriptions[id]
	if !ok {
		s.logger.Info("could not find remote track for subscribe ID", "subscribeID", id)
		return nil, false
	}
	return sub.remoteTrack, true
}

func (s *Session) subscribeIDForTrackAlias(alias uint64) (uint64, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	id, ok := s.trackAliasToSusbcribeID[alias]
	if !ok {
		s.logger.Info("could not find remote track for track alias", "trackAlias", alias)
		return 0, false
	}
	return id, true
}

func (s *Session) RemoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	id, ok := s.subscribeIDForTrackAlias(alias)
	if !ok {
		return nil, false
	}
	return s.RemoteTrackBySubscribeID(id)
}

// Local API to trigger outgoing control messages

func (s *Session) Subscribe(
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

func (s *Session) Announce(
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
	s.pendingOutgoingAnnouncements.Insert(namespace, Announcement{
		namespace:  namespace,
		parameters: map[uint64]wire.Parameter{},
	})
	return nil
}

func (s *Session) SubscribeAnnounces() {

}

func (s *Session) Fetch() {

}

func (s *Session) AcceptSubscription(sub Subscription) error {
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

func (s *Session) RejectSubscription(sub Subscription, errorCode uint64, reason string) error {
	return s.queueControlMessage(&wire.SubscribeErrorMessage{
		SubscribeID:  sub.ID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
		TrackAlias:   sub.TrackAlias,
	})
}

func (s *Session) AcceptAnnouncement(a Announcement) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.queueControlMessage(&wire.AnnounceOkMessage{
		TrackNamespace: a.namespace,
	}); err != nil {
		return err
	}
	s.incomingAnnouncements.Insert(a.namespace, a)
	return nil
}

// Remote API for handling incoming control messages

func (s *Session) OnControlMessage(msg wire.ControlMessage) error {
	if s.setupDone {
		switch m := msg.(type) {

		case *wire.SubscribeUpdateMessage:
			return s.subscribeUpdate(m)

		case *wire.SubscribeMessage:
			return s.subscribe(m)

		case *wire.SubscribeOkMessage:
			return s.subscribeOk(m)

		case *wire.SubscribeErrorMessage:
			return s.subscribeError(m)

		case *wire.AnnounceMessage:
			return s.announce(m)

		case *wire.AnnounceOkMessage:
			return s.announceOk(m)

		case *wire.AnnounceErrorMessage:
			return s.announceError(m)

		case *wire.UnannounceMessage:
			return s.unannounce(m)

		case *wire.UnsubscribeMessage:
			return s.unsubscribe(m)

		case *wire.SubscribeDoneMessage:
			return s.subscribeDone(m)

		case *wire.AnnounceCancelMessage:
			return s.announceCancel(m)

		case *wire.TrackStatusRequestMessage:
			return s.trackStatusRequest(m)

		case *wire.TrackStatusMessage:
			return s.trackStatus(m)

		case *wire.GoAwayMessage:
			return s.goAway(m)

		case *wire.SubscribeAnnouncesMessage:
			return s.subscribeAnnounces(m)

		case *wire.SubscribeAnnouncesOkMessage:
			return s.subscribeAnnouncesOk(m)

		case *wire.SubscribeAnnouncesErrorMessage:
			return s.subscribeAnnouncesError(m)

		case *wire.UnsubscribeAnnouncesMessage:
			return s.unsubscribeAnnounces(m)

		case *wire.MaxSubscribeIDMessage:
			return s.maxSubscribeID(m)

		case *wire.FetchMessage:
			return s.fetch(m)

		case *wire.FetchCancelMessage:
			return s.fetchCancel(m)

		case *wire.FetchOkMessage:
			return s.fetchOk(m)

		case *wire.FetchErrorMessage:
			return s.fetchError(m)
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

func (s *Session) clientSetup(m *wire.ClientSetupMessage) (err error) {
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

	s.queueControlMessage(&wire.ServerSetupMessage{
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
	})
	s.setupDone = true
	return nil
}

func (s *Session) serverSetup(m *wire.ServerSetupMessage) (err error) {
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

func (s *Session) subscribeUpdate(msg *wire.SubscribeUpdateMessage) error {
	return nil
}

func (s *Session) subscribe(msg *wire.SubscribeMessage) error {
	s.lock.Lock()
	defer s.lock.Unlock()

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
	select {
	case s.incomingSubscriptionQueue <- sub:
	default:
		panic("TODO pendingRemoteSubscriptionsOverflow")
	}
	return nil
}

func (s *Session) subscribeOk(msg *wire.SubscribeOkMessage) error {
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
		panic("TODO localSubscriptionResponsesOverflow")
	}
	return nil
}

func (s *Session) subscribeError(msg *wire.SubscribeErrorMessage) error {
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
		panic("TODO localSubscriptionResponsesOverflow")
	}
	return nil
}

func (s *Session) announce(msg *wire.AnnounceMessage) error {
	s.lock.Lock()
	defer s.lock.Unlock()

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
	select {
	case s.incomingAnnouncementQueue <- Announcement{
		namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
	}:
	default:
		panic("TODO pendingRemoteAnnouncementsQueueOverflow")
	}
	return nil
}

func (s *Session) announceOk(msg *wire.AnnounceOkMessage) error {
	return nil
}

func (s *Session) announceError(msg *wire.AnnounceErrorMessage) error {
	return nil
}

func (s *Session) unannounce(msg *wire.UnannounceMessage) error {
	return nil
}

func (s *Session) unsubscribe(msg *wire.UnsubscribeMessage) error {
	return nil
}

func (s *Session) subscribeDone(msg *wire.SubscribeDoneMessage) error {
	return nil
}

func (s *Session) announceCancel(msg *wire.AnnounceCancelMessage) error {
	return nil
}

func (s *Session) trackStatusRequest(msg *wire.TrackStatusRequestMessage) error {
	return nil
}

func (s *Session) trackStatus(msg *wire.TrackStatusMessage) error {
	return nil
}

func (s *Session) goAway(msg *wire.GoAwayMessage) error {
	return nil
}

func (s *Session) subscribeAnnounces(msg *wire.SubscribeAnnouncesMessage) error {
	return nil
}

func (s *Session) subscribeAnnouncesOk(msg *wire.SubscribeAnnouncesOkMessage) error {
	return nil
}

func (s *Session) subscribeAnnouncesError(msg *wire.SubscribeAnnouncesErrorMessage) error {
	return nil
}

func (s *Session) unsubscribeAnnounces(msg *wire.UnsubscribeAnnouncesMessage) error {
	return nil
}

func (s *Session) maxSubscribeID(msg *wire.MaxSubscribeIDMessage) error {
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

func (s *Session) fetch(msg *wire.FetchMessage) error {
	return nil
}

func (s *Session) fetchCancel(msg *wire.FetchCancelMessage) error {
	return nil
}

func (s *Session) fetchOk(msg *wire.FetchOkMessage) error {
	return nil
}

func (s *Session) fetchError(msg *wire.FetchErrorMessage) error {
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
