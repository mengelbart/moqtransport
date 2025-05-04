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

func (m *trackStatusRequestMap) add(tsr *trackStatusRequest) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.requests[tsr.requestID] = tsr
}

func (m *trackStatusRequestMap) delete(requestID uint64) (*trackStatusRequest, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	tsr, ok := m.requests[requestID]
	return tsr, ok
}
