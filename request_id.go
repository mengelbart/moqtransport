package moqtransport

import "sync/atomic"

type requestID struct {
	last atomic.Uint64
}

func newRequestID(perspective Perspective) *requestID {
	rid := &requestID{
		last: atomic.Uint64{},
	}
	if perspective == PerspectiveServer {
		rid.last.Store(1)
	}
	return rid
}

func (id *requestID) next() uint64 {
	return id.last.Add(2)
}
