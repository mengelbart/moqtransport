package moqtransport

import (
	"iter"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type controlStream struct {
	stream Stream
	logger *slog.Logger
}

func (s *controlStream) read() iter.Seq2[wire.ControlMessage, error] {
	parser := wire.NewControlMessageParser(s.stream)
	return func(yield func(wire.ControlMessage, error) bool) {
		for {
			msg, err := parser.Parse()
			if !yield(msg, err) {
				return
			}
		}
	}
}

func (s *controlStream) write(msg wire.ControlMessage) error {
	buf, err := compileMessage(msg)
	if err != nil {
		return err
	}
	s.logger.Info("sending message", "type", msg.Type().String(), "msg", msg)
	_, err = s.stream.Write(buf)
	if err != nil {
		return err
	}
	return nil
}
