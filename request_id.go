package moqtransport

import (
	"sync"
)

type requestIDGenerator struct {
	lock     sync.Mutex
	id       uint64
	interval uint64
}

func newRequestIDGenerator(initialID uint64) *requestIDGenerator {
	return &requestIDGenerator{
		id:       initialID,
		interval: 2,
	}
}

func (g *requestIDGenerator) next() uint64 {
	g.lock.Lock()
	defer g.lock.Unlock()
	next := g.id
	g.id += g.interval
	return next
}
