package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type localControlStream struct {
	w controlMessageWriter
}

func newLocalControlStream(w controlMessageWriter) *localControlStream {
	return &localControlStream{w: w}
}

func (s *localControlStream) write(msg wire.ControlMessage) error {
	return s.w.Write(msg)
}
