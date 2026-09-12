package moqtransport

import (
	"context"
	"errors"
	"sync"
)

var errDuplicateTrackAlias = errors.New("track alias already in use")

type objectReceiver interface {
	push(*Object)
	addSubgroupStream(*subgroupStream)
	removeSubgroupStream(*subgroupStream)
}

// A trackEntry is the state of one track alias. It exists from the first
// object, subgroup stream or SUBSCRIBE_OK for the alias until the receiver is
// removed.
type trackEntry struct {
	lock     sync.Mutex
	receiver objectReceiver
	pending  []*Object
	bound    chan struct{}
}

func newTrackEntry() *trackEntry {
	return &trackEntry{
		bound: make(chan struct{}),
	}
}

func (e *trackEntry) bind(r objectReceiver) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.receiver != nil {
		return errDuplicateTrackAlias
	}
	e.receiver = r
	for _, o := range e.pending {
		r.push(o)
	}
	e.pending = nil
	close(e.bound)
	return nil
}

func (e *trackEntry) boundTo(r objectReceiver) bool {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.receiver == r
}

// pushDatagram delivers o to the receiver, or buffers it until the alias is
// bound. It reports false if the buffer is full and o was dropped.
func (e *trackEntry) pushDatagram(o *Object, maxPending int) bool {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.receiver != nil {
		e.receiver.push(o)
		return true
	}
	if len(e.pending) >= maxPending {
		return false
	}
	e.pending = append(e.pending, o)
	return true
}

// waitForReceiver blocks until the alias is bound or ctx is done.
func (e *trackEntry) waitForReceiver(ctx context.Context) (objectReceiver, error) {
	select {
	case <-e.bound:
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.receiver, nil
}
