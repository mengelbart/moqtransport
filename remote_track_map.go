package moqtransport

import (
	"errors"
	"fmt"
	"sync"
)

type errSubscribesBlocked struct {
	maxSubscribeID uint64
}

func (e errSubscribesBlocked) Error() string {
	return fmt.Sprintf("too many subscribes, max_subscribe_id=%v", e.maxSubscribeID)
}

var (
	errMaxTrackAliasReached    = errors.New("max track alias reached")
	errDuplicateSubscribeIDBug = errors.New("internal error: duplicate subscribe ID")
	errDuplicateTrackAliasBug  = errors.New("internal error: duplicate track alias")
)

type remoteTrackMap struct {
	lock                    sync.Mutex
	maxSubscribeID          uint64
	nextSusbcribeID         uint64
	nextTrackAlias          uint64
	pending                 map[uint64]*RemoteTrack
	open                    map[uint64]*RemoteTrack
	trackAliasToSubscribeID map[uint64]uint64
}

func newRemoteTrackMap(maxID uint64) *remoteTrackMap {
	return &remoteTrackMap{
		lock:                    sync.Mutex{},
		maxSubscribeID:          maxID,
		nextSusbcribeID:         0,
		nextTrackAlias:          0,
		pending:                 map[uint64]*RemoteTrack{},
		open:                    map[uint64]*RemoteTrack{},
		trackAliasToSubscribeID: map[uint64]uint64{},
	}
}

func (m *remoteTrackMap) updateMaxSubscribeID(next uint64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.maxSubscribeID > 0 && next <= m.maxSubscribeID {
		return errMaxSubscribeIDDecreased
	}
	m.maxSubscribeID = next
	return nil
}

func (m *remoteTrackMap) findBySubscribeID(id uint64) (*RemoteTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub, ok := m.open[id]
	if !ok {
		sub, ok = m.pending[id]
	}
	if !ok {
		return nil, false
	}
	return sub, true
}

// newID generates the next subscribe ID. It must be called while holding the
// lock.
func (m *remoteTrackMap) newID() (uint64, error) {
	if m.nextSusbcribeID >= m.maxSubscribeID {
		return 0, errSubscribesBlocked{
			maxSubscribeID: m.maxSubscribeID,
		}
	}
	id := m.nextSusbcribeID
	m.nextSusbcribeID++
	return id, nil
}

// newAlias generates the next track alias. It must be called while holding the
// lock.
func (m *remoteTrackMap) newAlias() (uint64, error) {
	if m.nextTrackAlias >= maxVarint {
		return 0, errMaxTrackAliasReached
	}
	alias := m.nextTrackAlias
	m.nextTrackAlias++
	return alias, nil
}

func (m *remoteTrackMap) addPending(rt *RemoteTrack) (id uint64, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	id, err = m.newID()
	if err != nil {
		return 0, err
	}
	if _, ok := m.pending[id]; ok {
		// Should never happen
		return 0, errDuplicateSubscribeIDBug
	}
	if _, ok := m.open[id]; ok {
		// Should never happen
		return 0, errDuplicateSubscribeIDBug
	}
	m.pending[id] = rt
	return id, nil
}

func (m *remoteTrackMap) addPendingWithAlias(rt *RemoteTrack) (id, alias uint64, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	id, err = m.newID()
	if err != nil {
		return 0, 0, err
	}
	alias, err = m.newAlias()
	if err != nil {
		return 0, 0, err
	}
	if _, ok := m.pending[id]; ok {
		// Should never happen
		return 0, 0, errDuplicateSubscribeIDBug
	}
	if _, ok := m.open[id]; ok {
		// Should never happen
		return 0, 0, errDuplicateSubscribeIDBug
	}
	if _, ok := m.trackAliasToSubscribeID[alias]; ok {
		// Should never happen
		return 0, 0, errDuplicateTrackAliasBug
	}
	m.pending[id] = rt
	m.trackAliasToSubscribeID[alias] = id
	return id, alias, nil
}

func (m *remoteTrackMap) confirm(id uint64) (*RemoteTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	s, ok := m.pending[id]
	if !ok {
		return nil, false
	}
	delete(m.pending, id)
	m.open[id] = s
	return s, true
}

func (m *remoteTrackMap) reject(id uint64) (*RemoteTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	s, ok := m.pending[id]
	if !ok {
		return nil, false
	}
	delete(m.pending, id)
	return s, true
}

func (m *remoteTrackMap) findByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	m.lock.Lock()
	id, ok := m.trackAliasToSubscribeID[alias]
	m.lock.Unlock()
	if !ok {
		return nil, false
	}
	return m.findBySubscribeID(id)
}
