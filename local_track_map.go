package moqtransport

import "sync"

type localTrackMap struct {
	lock    sync.Mutex
	pending map[uint64]*localTrack
	open    map[uint64]*localTrack
}

func newLocalTrackMap(maxSubscribeID uint64) *localTrackMap {
	return &localTrackMap{
		lock:    sync.Mutex{},
		pending: map[uint64]*localTrack{},
		open:    map[uint64]*localTrack{},
	}
}

func (m *localTrackMap) addPending(lt *localTrack) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.pending[lt.subscribeID]; ok {
		return errDuplicateSubscribeID
	}
	if _, ok := m.open[lt.subscribeID]; ok {
		return errDuplicateSubscribeID
	}
	m.pending[lt.subscribeID] = lt
	return nil
}

func (m *localTrackMap) delete(id uint64) {

}

func (m *localTrackMap) confirm(id uint64) {

}
