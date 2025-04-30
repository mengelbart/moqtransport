package moqtransport

import (
	"errors"
	"fmt"
	"sync"
)

type errRequestsBlocked struct {
	maxRequestID uint64
}

func (e errRequestsBlocked) Error() string {
	return fmt.Sprintf("too many subscribes, max_request_id=%v", e.maxRequestID)
}

var (
	errMaxTrackAliasReached   = errors.New("max track alias reached")
	errDuplicateRequestIDBug  = errors.New("internal error: duplicate request ID")
	errDuplicateTrackAliasBug = errors.New("internal error: duplicate track alias")
)

type remoteTrackMap struct {
	lock                  sync.Mutex
	maxRequestID          uint64
	nextRequestID         uint64
	nextTrackAlias        uint64
	pending               map[uint64]*RemoteTrack
	open                  map[uint64]*RemoteTrack
	trackAliasToRequestID map[uint64]uint64
}

func newRemoteTrackMap(maxID uint64) *remoteTrackMap {
	return &remoteTrackMap{
		lock:                  sync.Mutex{},
		maxRequestID:          maxID,
		nextRequestID:         0,
		nextTrackAlias:        0,
		pending:               map[uint64]*RemoteTrack{},
		open:                  map[uint64]*RemoteTrack{},
		trackAliasToRequestID: map[uint64]uint64{},
	}
}

func (m *remoteTrackMap) updateMaxRequestID(next uint64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.maxRequestID > 0 && next <= m.maxRequestID {
		return errMaxRequestIDDecreased
	}
	m.maxRequestID = next
	return nil
}

func (m *remoteTrackMap) findByRequestID(id uint64) (*RemoteTrack, bool) {
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

// newID generates the next request ID. It must be called while holding the
// lock.
func (m *remoteTrackMap) newID() (uint64, error) {
	if m.nextRequestID >= m.maxRequestID {
		return 0, errRequestsBlocked{
			maxRequestID: m.maxRequestID,
		}
	}
	id := m.nextRequestID
	m.nextRequestID++
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
		return 0, errDuplicateRequestIDBug
	}
	if _, ok := m.open[id]; ok {
		// Should never happen
		return 0, errDuplicateRequestIDBug
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
		return 0, 0, errDuplicateRequestIDBug
	}
	if _, ok := m.open[id]; ok {
		// Should never happen
		return 0, 0, errDuplicateRequestIDBug
	}
	if _, ok := m.trackAliasToRequestID[alias]; ok {
		// Should never happen
		return 0, 0, errDuplicateTrackAliasBug
	}
	m.pending[id] = rt
	m.trackAliasToRequestID[alias] = id
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
	id, ok := m.trackAliasToRequestID[alias]
	m.lock.Unlock()
	if !ok {
		return nil, false
	}
	return m.findByRequestID(id)
}
