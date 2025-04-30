package moqtransport

import "sync/atomic"

type sequence struct {
	last     atomic.Uint64
	interval uint64
}

func newSequence(initial, interval uint64) *sequence {
	s := &sequence{
		last:     atomic.Uint64{},
		interval: interval,
	}
	s.last.Store(initial)
	return s
}

func (s *sequence) next() uint64 {
	return s.last.Add(s.interval) - s.interval
}
