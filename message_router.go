package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type sink interface {
	push(*objectMessage) error
}

type controlMsgSender interface {
	send(message) error
}

type messageRouter struct {
	ctx context.Context

	logger *slog.Logger

	conn connection

	controlMsgSender controlMsgSender

	receiveTrackLock sync.RWMutex
	receiveTracks    map[uint64]sink

	sendTrackLock sync.RWMutex
	sendTracks    map[string]*SendTrack

	subscriptionCh chan *Subscription
	announcementCh chan *Announcement

	transactionsLock sync.RWMutex
	transactions     map[messageKey]*transaction
}

type transaction struct {
	keyedMessage
	responseCh chan message
}

func newMessageRouter(conn connection, cms controlMsgSender) *messageRouter {
	return &messageRouter{
		ctx:              context.Background(),
		logger:           defaultLogger.With(componentKey, "MOQ_MESSAGE_ROUTER"),
		conn:             conn,
		controlMsgSender: cms,
		receiveTrackLock: sync.RWMutex{},
		receiveTracks:    map[uint64]sink{},
		sendTrackLock:    sync.RWMutex{},
		sendTracks:       map[string]*SendTrack{},
		subscriptionCh:   make(chan *Subscription),
		announcementCh:   make(chan *Announcement),
		transactionsLock: sync.RWMutex{},
		transactions:     map[messageKey]*transaction{},
	}
}

func (s *messageRouter) handleControlMessage(msg message) error {
	var err error
	switch m := msg.(type) {
	case *objectMessage:
		return &moqError{
			code:    protocolViolationErrorCode,
			message: "received object message on control stream",
		}
	case *subscribeRequestMessage:
		err = s.controlMsgSender.send(s.handleSubscribeRequest(m))
	case *subscribeOkMessage:
		err = s.handleTransactionResponse(m)
	case *subscribeErrorMessage:
		err = s.handleTransactionResponse(m)
	case *subscribeFinMessage:
		panic("TODO")
	case *subscribeRstMessage:
		panic("TODO")
	case *unsubscribeMessage:
		panic("TODO")
	case *announceMessage:
		err = s.controlMsgSender.send(s.handleAnnounceMessage(m))
	case *announceOkMessage:
		err = s.handleTransactionResponse(m)
	case *announceErrorMessage:
		err = s.handleTransactionResponse(m)
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

func (s *messageRouter) handleTransactionResponse(msg keyedMessage) error {
	s.transactionsLock.RLock()
	t, ok := s.transactions[msg.key()]
	s.transactionsLock.RUnlock()
	if ok {
		select {
		case t.responseCh <- msg:
		case <-s.ctx.Done():
		}
		return nil
	}
	return &moqError{
		code:    genericErrorErrorCode,
		message: "received transaction response message to an unknown transaction",
	}
}

func (s *messageRouter) handleSubscribeRequest(msg *subscribeRequestMessage) message {
	sub := &Subscription{
		lock:        sync.RWMutex{},
		track:       newSendTrack(s.conn),
		responseCh:  make(chan error),
		closeCh:     make(chan struct{}),
		expires:     0,
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
			TrackNamespace: sub.namespace,
			TrackName:      sub.trackname,
			ErrorCode:      0, // TODO: set correct error code
			ReasonPhrase:   "session closed",
		}
	case s.subscriptionCh <- sub:
	}
	select {
	case <-s.ctx.Done():
		close(sub.closeCh)
	case err := <-sub.responseCh:
		if err != nil {
			return &subscribeErrorMessage{
				TrackNamespace: sub.namespace,
				TrackName:      sub.trackname,
				ErrorCode:      0, // TODO: Implement a custom error type including the code?
				ReasonPhrase:   fmt.Sprintf("subscription rejected: %v", err),
			}
		}
	}
	sub.lock.RLock()
	defer sub.lock.RUnlock()
	s.sendTrackLock.Lock()
	defer s.sendTrackLock.Unlock()
	s.sendTracks[msg.key().id] = sub.track
	return &subscribeOkMessage{
		TrackNamespace: sub.namespace,
		TrackName:      sub.trackname,
		TrackID:        sub.TrackID(),
		Expires:        sub.expires,
	}
}

func (s *messageRouter) handleAnnounceMessage(msg *announceMessage) message {
	a := &Announcement{
		responseCh: make(chan error),
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
	case err := <-a.responseCh:
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
	o, ok := m.(*objectMessage)
	if !ok {
		return &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected control message on object stream",
		}
	}
	s.receiveTrackLock.RLock()
	t, ok := s.receiveTracks[o.TrackID]
	s.receiveTrackLock.RUnlock()
	if ok {
		return t.push(o)
	}
	s.logger.Warn("dropping object message for unknown track")
	return nil
}

func (s *messageRouter) readSubscription(ctx context.Context) (*Subscription, error) {
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

func (s *messageRouter) subscribe(ctx context.Context, namespace, trackname, auth string) (*ReceiveTrack, error) {
	sm := &subscribeRequestMessage{
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
	responseCh := make(chan message)
	t := &transaction{
		keyedMessage: sm,
		responseCh:   responseCh,
	}
	s.transactionsLock.Lock()
	s.transactions[sm.key()] = t
	s.transactionsLock.Unlock()
	if err := s.controlMsgSender.send(sm); err != nil {
		return nil, err
	}
	var resp message
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case resp = <-responseCh:
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		if v.key().id != sm.key().id {
			s.logger.Error("internal error: received response message for wrong subscribe transaction key.", "expected_key_id", sm.key().id, "repsonse_key_id", v.key().id)
			return nil, &moqError{
				code:    genericErrorErrorCode,
				message: "internal error",
			}
		}
		t := newReceiveTrack()
		s.receiveTrackLock.Lock()
		s.receiveTracks[v.TrackID] = t
		s.receiveTrackLock.Unlock()
		return t, nil

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
	responseCh := make(chan message)
	t := &transaction{
		keyedMessage: am,
		responseCh:   responseCh,
	}
	s.transactionsLock.Lock()
	s.transactions[am.key()] = t
	s.transactionsLock.Unlock()
	if err := s.controlMsgSender.send(am); err != nil {
		return err
	}
	var resp message
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	case resp = <-responseCh:
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		if v.TrackNamespace != am.TrackNamespace {
			s.logger.Error("internal error: received response message for wrong announce transaction namespace.", "expected_namespace", am.TrackNamespace, "response_namespace", v.TrackNamespace)
			return &moqError{
				code:    genericErrorErrorCode,
				message: "internal error",
			}
		}
		return nil
	case *announceErrorMessage:
		return errors.New(v.ReasonPhrase)
	}
	return &moqError{
		code:    genericErrorErrorCode,
		message: "received unexpected response message type to announceMessage",
	}
}
