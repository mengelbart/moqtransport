package moqtransport

import (
	"sync"
)

type localTrackMap struct {
	lock    sync.Mutex
	pending map[uint64]*localTrack
	open    map[uint64]*localTrack
}

func newLocalTrackMap() *localTrackMap {
	return &localTrackMap{
		lock:    sync.Mutex{},
		pending: map[uint64]*localTrack{},
		open:    map[uint64]*localTrack{},
	}
}

func (m *localTrackMap) addPending(lt *localTrack) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.pending[lt.subscribeID]; ok {
		return false
	}
	if _, ok := m.open[lt.subscribeID]; ok {
		return false
	}
	m.pending[lt.subscribeID] = lt
	return true
}

func (m *localTrackMap) findByID(id uint64) (*localTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub, ok := m.pending[id]
	if !ok {
		sub, ok = m.open[id]
	}
	return sub, ok
}

func (m *localTrackMap) delete(id uint64) (*localTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub, ok := m.pending[id]
	if !ok {
		sub, ok = m.open[id]
	}
	if !ok {
		return nil, false
	}
	delete(m.pending, id)
	delete(m.open, id)
	return sub, true
}

func (m *localTrackMap) confirm(id uint64) (*localTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	lt, ok := m.pending[id]
	if !ok {
		return nil, false
	}
	delete(m.pending, id)
	m.open[id] = lt
	return lt, true
}

func (m *localTrackMap) reject(id uint64) (*localTrack, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	lt, ok := m.pending[id]
	if !ok {
		return nil, false
	}
	delete(m.pending, id)
	return lt, true
}
