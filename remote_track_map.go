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
	errUnknownRequestID       = errors.New("unknown request ID")
	errDuplicateRequestIDBug  = errors.New("internal error: duplicate request ID")
	errDuplicateTrackAliasBug = errors.New("internal error: duplicate track alias")
)

type remoteTrackMap struct {
	lock                  sync.Mutex
	nextTrackAlias        uint64
	pending               map[uint64]*RemoteTrack
	open                  map[uint64]*RemoteTrack
	trackAliasToRequestID map[uint64]uint64
}

func newRemoteTrackMap() *remoteTrackMap {
	return &remoteTrackMap{
		lock:                  sync.Mutex{},
		nextTrackAlias:        0,
		pending:               map[uint64]*RemoteTrack{},
		open:                  map[uint64]*RemoteTrack{},
		trackAliasToRequestID: map[uint64]uint64{},
	}
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

func (m *remoteTrackMap) addPending(requestID uint64, rt *RemoteTrack) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.pending[requestID]; ok {
		// Should never happen
		return errDuplicateRequestIDBug
	}
	if _, ok := m.open[requestID]; ok {
		// Should never happen
		return errDuplicateRequestIDBug
	}
	m.pending[requestID] = rt
	return nil
}

func (m *remoteTrackMap) addPendingWithAlias(requestID uint64, rt *RemoteTrack) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.pending[requestID]; ok {
		return errDuplicateRequestIDBug
	}
	if _, ok := m.open[requestID]; ok {
		return errDuplicateRequestIDBug
	}
	m.pending[requestID] = rt
	return nil
}

func (m *remoteTrackMap) setAlias(id, alias uint64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.trackAliasToRequestID[alias]; ok {
		return errDuplicateTrackAliasBug
	}
	m.trackAliasToRequestID[alias] = id
	return nil
}

func (m *remoteTrackMap) confirm(id uint64) (*RemoteTrack, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	s, ok := m.pending[id]
	if !ok {
		return nil, errUnknownRequestID
	}
	delete(m.pending, id)
	m.open[id] = s
	return s, nil
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
