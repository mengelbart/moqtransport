package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire2"

type localControlStream struct {
	w controlMessageWriter
}

func newLocalControlStream(w controlMessageWriter) *localControlStream {
	return &localControlStream{w: w}
}

func (s *localControlStream) write(msg wire2.ControlMessage) error {
	return s.w.Write(msg)
}
