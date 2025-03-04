package moqtransport

import (
	"fmt"
	"sync"
)

type errMaxSusbcribeIDViolation struct {
	maxSubscribeID uint64
}

func (e errMaxSusbcribeIDViolation) Error() string {
	return fmt.Sprintf("too many subscribes, max_subscribe_id=%v", e.maxSubscribeID)
}

type subscriptionMap struct {
	lock                    sync.Mutex
	maxSubscribeID          uint64
	pendingSubscriptions    map[uint64]*subscription
	subscriptions           map[uint64]*subscription
	trackAliasToSusbcribeID map[uint64]uint64
}

func newSubscriptionMap(maxID uint64) *subscriptionMap {
	return &subscriptionMap{
		lock:                    sync.Mutex{},
		maxSubscribeID:          maxID,
		pendingSubscriptions:    map[uint64]*subscription{},
		subscriptions:           map[uint64]*subscription{},
		trackAliasToSusbcribeID: map[uint64]uint64{},
	}
}

func (m *subscriptionMap) getMaxSubscribeID() uint64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.maxSubscribeID
}

func (m *subscriptionMap) updateMaxSubscribeID(next uint64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.maxSubscribeID > 0 && next <= m.maxSubscribeID {
		return errMaxSubscribeIDDecreased
	}
	m.maxSubscribeID = next
	return nil
}

func (m *subscriptionMap) addPending(s *subscription) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if s.ID >= m.maxSubscribeID {
		return errMaxSusbcribeIDViolation{
			maxSubscribeID: m.maxSubscribeID,
		}
	}
	if _, ok := m.pendingSubscriptions[s.ID]; ok {
		return errDuplicateSubscribeID
	}
	if _, ok := m.subscriptions[s.ID]; ok {
		return errDuplicateSubscribeID
	}
	m.pendingSubscriptions[s.ID] = s
	m.trackAliasToSusbcribeID[s.TrackAlias] = s.ID
	return nil
}

func (m *subscriptionMap) confirm(id uint64) (*subscription, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.pendingSubscriptions[id]
	if !ok {
		return nil, ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: fmt.Sprintf("unknown subscribe ID: %v", id),
		}
	}
	s := m.pendingSubscriptions[id]
	delete(m.pendingSubscriptions, id)
	m.subscriptions[id] = s
	return s, nil
}

func (m *subscriptionMap) hasPending(id uint64) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.pendingSubscriptions[id]
	return ok
}

func (m *subscriptionMap) reject(id uint64) (*subscription, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub, ok := m.pendingSubscriptions[id]
	if !ok {
		return nil, ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "unknown subscribe ID",
		}
	}
	delete(m.pendingSubscriptions, id)
	return sub, nil
}

func (m *subscriptionMap) delete(id uint64) (*subscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub, ok := m.pendingSubscriptions[id]
	if !ok {
		sub, ok = m.subscriptions[id]
	}
	if !ok {
		return nil, false
	}
	delete(m.pendingSubscriptions, id)
	delete(m.subscriptions, id)
	delete(m.trackAliasToSusbcribeID, sub.TrackAlias)
	return sub, true
}

func (m *subscriptionMap) findBySubscribeID(id uint64) (*subscription, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub, ok := m.subscriptions[id]
	if !ok {
		sub, ok = m.pendingSubscriptions[id]
	}
	if !ok {
		return nil, false
	}
	return sub, true
}

func (m *subscriptionMap) findByTrackAlias(alias uint64) (*subscription, bool) {
	m.lock.Lock()
	id, ok := m.trackAliasToSusbcribeID[alias]
	m.lock.Unlock()
	if !ok {
		return nil, false
	}
	return m.findBySubscribeID(id)
}
