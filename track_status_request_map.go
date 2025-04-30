package moqtransport

import (
	"sync"
)

type trackStatusRequestMap struct {
	lock     sync.Mutex
	requests map[uint64]*trackStatusRequest
}

func newTrackStatusRequestMap() *trackStatusRequestMap {
	return &trackStatusRequestMap{
		lock:     sync.Mutex{},
		requests: map[uint64]*trackStatusRequest{},
	}
}

func (m *trackStatusRequestMap) add(tsr *trackStatusRequest) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.requests[tsr.requestID] = tsr
	return true
}

func (m *trackStatusRequestMap) delete(rid uint64) (*trackStatusRequest, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	tsr, ok := m.requests[rid]
	return tsr, ok
}
