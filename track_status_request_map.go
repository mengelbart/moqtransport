package moqtransport

import (
	"strings"
	"sync"
)

type trackStatusRequestMap struct {
	lock     sync.Mutex
	requests map[string]*trackStatusRequest
}

func newTrackStatusRequestMap() *trackStatusRequestMap {
	return &trackStatusRequestMap{
		lock:     sync.Mutex{},
		requests: map[string]*trackStatusRequest{},
	}
}

func (m *trackStatusRequestMap) add(tsr *trackStatusRequest) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	// TODO: Implement better mapping than just concatenating all namespace
	// elements and trackname?
	key := strings.Join(append(tsr.namespace, tsr.trackname), "")
	if _, ok := m.requests[key]; ok {
		return false
	}
	m.requests[key] = tsr
	return true
}

func (m *trackStatusRequestMap) delete(namespace []string, trackname string) (*trackStatusRequest, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	key := strings.Join(append(namespace, trackname), "")
	tsr, ok := m.requests[key]
	return tsr, ok
}
