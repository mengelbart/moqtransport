package moqtransport

import (
	"sync"
)

type requestIDGenerator struct {
	lock     sync.Mutex
	id       uint64
	interval uint64
}

func newRequestIDGenerator(initialID, maxID, interval uint64) *requestIDGenerator {
	return &requestIDGenerator{
		id:       initialID,
		interval: interval,
	}
}

func (g *requestIDGenerator) next() (uint64, error) {
	g.lock.Lock()
	defer g.lock.Unlock()
	next := g.id
	g.id += g.interval
	return next, nil
}
