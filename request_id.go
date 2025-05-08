package moqtransport

import (
	"errors"
	"sync"
)

var errRequestIDblocked = errors.New("request IDs blocked")

type requestIDGenerator struct {
	lock     sync.Mutex
	id       uint64
	max      uint64
	interval uint64
}

func newRequestIDGenerator(initialID, maxID, interval uint64) *requestIDGenerator {
	return &requestIDGenerator{
		id:       initialID,
		max:      maxID,
		interval: interval,
	}
}

func (g *requestIDGenerator) next() (uint64, error) {
	g.lock.Lock()
	defer g.lock.Unlock()
	if g.id >= g.max {
		return g.max, errRequestIDblocked
	}
	next := g.id
	g.id += g.interval
	return next, nil
}

func (g *requestIDGenerator) setMax(v uint64) error {
	g.lock.Lock()
	defer g.lock.Unlock()
	if v < g.max {
		return errMaxRequestIDDecreased
	}
	g.max = v
	return nil
}
