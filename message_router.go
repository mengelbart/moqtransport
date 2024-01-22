package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type controlMsgSender interface {
	send(message) error
}

type messageRouter struct {
	ctx              context.Context
	logger           *slog.Logger
	conn             connection
	controlMsgSender controlMsgSender

	subscriptionCh chan *SendSubscription
	announcementCh chan *Announcement

	// NEW
	// incoming subscription (i.e. has a sendTrack)
	activeSendSubscriptionsLock sync.RWMutex
	activeSendSubscriptions     map[uint64]*SendSubscription

	activeReceiveSubscriptionsLock sync.RWMutex
	activeReceiveSubscriptions     map[uint64]*ReceiveSubscription

	// outgoing subscriptions (i.e. has a receiveTrack)
	pendingSubscriptionsLock sync.RWMutex
	pendingSubscriptions     map[uint64]*ReceiveSubscription

	pendingAnnouncementsLock sync.RWMutex
	pendingAnnouncements     map[string]*announcement
}

type announcement struct {
	responseCh chan trackNamespacer
}

type subscribeIDer interface {
	message
	subscribeID() uint64
}

type trackNamespacer interface {
	message
	trackNamespace() string
}

func newMessageRouter(conn connection, cms controlMsgSender) *messageRouter {
	return &messageRouter{
		ctx:                            context.Background(),
		logger:                         defaultLogger.With(componentKey, "MOQ_MESSAGE_ROUTER"),
		conn:                           conn,
		controlMsgSender:               cms,
		subscriptionCh:                 make(chan *SendSubscription),
		announcementCh:                 make(chan *Announcement),
		activeSendSubscriptionsLock:    sync.RWMutex{},
		activeSendSubscriptions:        map[uint64]*SendSubscription{},
		activeReceiveSubscriptionsLock: sync.RWMutex{},
		activeReceiveSubscriptions:     map[uint64]*ReceiveSubscription{},
		pendingSubscriptionsLock:       sync.RWMutex{},
		pendingSubscriptions:           map[uint64]*ReceiveSubscription{},
		pendingAnnouncementsLock:       sync.RWMutex{},
		pendingAnnouncements:           map[string]*announcement{},
	}
}

func (s *messageRouter) handleControlMessage(msg message) error {
	var err error
	switch m := msg.(type) {
	case *objectStreamMessage:
		return &moqError{
			code:    protocolViolationErrorCode,
			message: "received object message on control stream",
		}
	case *subscribeMessage:
		err = s.controlMsgSender.send(s.handleSubscribeRequest(m))
	case *subscribeOkMessage:
		err = s.handleSubscriptionResponse(m)
	case *subscribeErrorMessage:
		err = s.handleSubscriptionResponse(m)
	case *subscribeFinMessage:
		panic("TODO")
	case *subscribeRstMessage:
		panic("TODO")
	case *unsubscribeMessage:
		panic("TODO")
	case *announceMessage:
		err = s.controlMsgSender.send(s.handleAnnounceMessage(m))
	case *announceOkMessage:
		err = s.handleAnnouncementResponse(m)
	case *announceErrorMessage:
		err = s.handleAnnouncementResponse(m)
	case *goAwayMessage:
		panic("TODO")
	default:
		return &moqError{
			code:    genericErrorErrorCode,
			message: "received unexpected message type on control stream",
		}
	}
	return err
}

func (s *messageRouter) handleSubscriptionResponse(msg subscribeIDer) error {
	s.pendingSubscriptionsLock.RLock()
	sub, ok := s.pendingSubscriptions[msg.subscribeID()]
	s.pendingSubscriptionsLock.RUnlock()
	if !ok {
		return &moqError{
			code:    genericErrorErrorCode,
			message: "received subscription response message to an unknown subscription",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case sub.responseCh <- msg:
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
	return nil
}

func (s *messageRouter) handleAnnouncementResponse(msg trackNamespacer) error {
	s.pendingAnnouncementsLock.RLock()
	a, ok := s.pendingAnnouncements[msg.trackNamespace()]
	s.pendingAnnouncementsLock.RUnlock()
	if !ok {
		return &moqError{
			code:    genericErrorErrorCode,
			message: "received announcement response message to an unknown announcement",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case a.responseCh <- msg:
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
	return nil
}

func (s *messageRouter) handleSubscribeRequest(msg *subscribeMessage) message {
	sub := &SendSubscription{
		lock:        sync.RWMutex{},
		responseCh:  make(chan error),
		closeCh:     make(chan struct{}),
		expires:     0,
		conn:        s.conn,
		subscribeID: msg.SubscribeID,
		trackAlias:  msg.TrackAlias,
		namespace:   msg.TrackNamespace,
		trackname:   msg.TrackName,
		startGroup:  msg.StartGroup,
		startObject: msg.StartObject,
		endGroup:    msg.EndGroup,
		endObject:   msg.EndObject,
		parameters:  msg.Parameters,
	}
	select {
	case <-s.ctx.Done():
		return &subscribeErrorMessage{
			SubscribeID:  msg.SubscribeID,
			ErrorCode:    0,
			ReasonPhrase: "session closed",
			TrackAlias:   msg.TrackAlias,
		}
	case s.subscriptionCh <- sub:
	}
	select {
	case <-s.ctx.Done():
		close(sub.closeCh)
	case err := <-sub.responseCh:
		if err != nil {
			return &subscribeErrorMessage{
				SubscribeID:  msg.SubscribeID,
				ErrorCode:    0,
				ReasonPhrase: fmt.Sprintf("subscription rejected: %v", err),
				TrackAlias:   msg.TrackAlias,
			}
		}
	}
	return &subscribeOkMessage{
		SubscribeID: msg.SubscribeID,
		Expires:     sub.expires,
	}
}

func (s *messageRouter) handleAnnounceMessage(msg *announceMessage) message {
	a := &Announcement{
		errorCh:    make(chan error),
		closeCh:    make(chan struct{}),
		namespace:  msg.TrackNamespace,
		parameters: msg.TrackRequestParameters,
	}
	select {
	case <-s.ctx.Done():
		return &announceErrorMessage{
			TrackNamespace: msg.TrackNamespace,
			ErrorCode:      0, // TODO: Set correct code?
			ReasonPhrase:   "session closed",
		}
	case s.announcementCh <- a:
	}
	select {
	case <-s.ctx.Done():
		close(a.closeCh)
	case err := <-a.errorCh:
		if err != nil {
			return &announceErrorMessage{
				TrackNamespace: a.namespace,
				ErrorCode:      0, // TODO: Implement a custom error type including the code?
				ReasonPhrase:   fmt.Sprintf("announcement rejected: %v", err),
			}
		}
	}
	return &announceOkMessage{
		TrackNamespace: a.namespace,
	}
}

func (s *messageRouter) handleObjectMessage(m message) error {
	o, ok := m.(*objectStreamMessage)
	if !ok {
		return &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected control message on object stream",
		}
	}
	s.activeReceiveSubscriptionsLock.RLock()
	sub, ok := s.activeReceiveSubscriptions[o.SubscribeID]
	s.activeReceiveSubscriptionsLock.RUnlock()
	if ok {
		_, err := sub.push(o)
		return err
	}
	s.logger.Warn("dropping object message for unknown track")
	return nil
}

func (s *messageRouter) readSubscription(ctx context.Context) (*SendSubscription, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("session closed") // TODO: Better error message including a reason?
	case s := <-s.subscriptionCh:
		return s, nil
	}
}

func (s *messageRouter) readAnnouncement(ctx context.Context) (*Announcement, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("session closed") // TODO: Better error message including a reason?
	case a := <-s.announcementCh:
		return a, nil
	}
}

func (s *messageRouter) subscribe(ctx context.Context, subscribeID, trackAlias uint64, namespace, trackname, auth string) (*ReceiveSubscription, error) {
	sm := &subscribeMessage{
		SubscribeID:    subscribeID,
		TrackAlias:     trackAlias,
		TrackNamespace: namespace,
		TrackName:      trackname,
		StartGroup:     Location{},
		StartObject:    Location{},
		EndGroup:       Location{},
		EndObject:      Location{},
		Parameters:     map[uint64]parameter{},
	}
	if len(auth) > 0 {
		sm.Parameters[authorizationParameterKey] = stringParameter{
			k: authorizationParameterKey,
			v: auth,
		}
	}
	sub := newReceiveSubscription()
	s.pendingSubscriptionsLock.Lock()
	s.pendingSubscriptions[sm.subscribeID()] = sub
	s.pendingSubscriptionsLock.Unlock()
	if err := s.controlMsgSender.send(sm); err != nil {
		return nil, err
	}
	var resp subscribeIDer
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case resp = <-sub.responseCh:
	}
	if resp.subscribeID() != sm.SubscribeID {
		s.logger.Error("internal error: received response message for wrong subscription ID.", "expected_id", sm.SubscribeID, "repsonse_id", resp.subscribeID())
		return nil, &moqError{
			code:    genericErrorErrorCode,
			message: "internal error",
		}
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		s.activeReceiveSubscriptionsLock.Lock()
		s.activeReceiveSubscriptions[v.SubscribeID] = sub
		s.activeReceiveSubscriptionsLock.Unlock()
		return sub, nil
	case *subscribeErrorMessage:
		return nil, errors.New(v.ReasonPhrase)
	}
	return nil, &moqError{
		code:    genericErrorErrorCode,
		message: "received unexpected response message type to subscribeRequestMessage",
	}
}

func (s *messageRouter) announce(ctx context.Context, namespace string) error {
	if len(namespace) == 0 {
		return errors.New("invalid track namespace")
	}
	am := &announceMessage{
		TrackNamespace:         namespace,
		TrackRequestParameters: map[uint64]parameter{},
	}
	responseCh := make(chan trackNamespacer)
	t := &announcement{
		responseCh: responseCh,
	}
	s.pendingAnnouncementsLock.Lock()
	s.pendingAnnouncements[am.TrackNamespace] = t
	s.pendingAnnouncementsLock.Unlock()
	if err := s.controlMsgSender.send(am); err != nil {
		return err
	}
	var resp trackNamespacer
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	case resp = <-responseCh:
	}
	if resp.trackNamespace() != am.TrackNamespace {
		s.logger.Error("internal error: received response message for wrong announce track namespace.", "expected_track_namespace", am.TrackNamespace, "response_track_namespace", resp.trackNamespace())
		return &moqError{
			code:    genericErrorErrorCode,
			message: "internal error",
		}
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		return nil
	case *announceErrorMessage:
		return errors.New(v.ReasonPhrase)
	}
	return &moqError{
		code:    genericErrorErrorCode,
		message: "received unexpected response message type to announceMessage",
	}
}
