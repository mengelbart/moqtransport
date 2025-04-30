package moqtransport

import "sync/atomic"

type requestID struct {
	last atomic.Uint64
}

func (id *requestID) next() uint64 {
	return id.last.Add(2)
}
