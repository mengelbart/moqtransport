package moqtransport

import (
	"context"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
)

// localControlStream serializes writes on the control stream and holds them
// back until SETUP has been written.
type localControlStream struct {
	w     messageWriter
	lock  sync.Mutex
	ready chan struct{}
}

func newLocalControlStream(w messageWriter) *localControlStream {
	return &localControlStream{w: w, ready: make(chan struct{})}
}

func (s *localControlStream) writeSetup(msg *wire.Setup) error {
	defer close(s.ready)
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.w.Write(msg)
}

func (s *localControlStream) write(ctx context.Context, msg wire.ControlMessage) error {
	select {
	case <-s.ready:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.w.Write(msg)
}
