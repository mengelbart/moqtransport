package moqtransport

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type parser interface {
	Parse() (wire.Message, error)
}

type controlStream struct {
	logger    *slog.Logger
	stream    Stream
	handle    messageHandler
	parser    parser
	sendQueue chan wire.Message
	closeCh   chan struct{}
}

type messageHandler func(wire.Message) error

func newControlStream(s Stream, h messageHandler) *controlStream {
	cs := &controlStream{
		logger:    defaultLogger.WithGroup("MOQ_CONTROL_STREAM"),
		stream:    s,
		handle:    h,
		parser:    wire.NewControlMessageParser(s),
		sendQueue: make(chan wire.Message, 64),
		closeCh:   make(chan struct{}),
	}
	go cs.readMessages()
	go cs.writeMessages()
	return cs
}

func (s *controlStream) readMessages() {
	for {
		msg, err := s.parser.Parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			s.logger.Error("TODO", "error", err)
			return
		}
		if err = s.handle(msg); err != nil {
			s.logger.Error("failed to handle control stream message", "error", err, "msg", msg)
			panic("TODO: Close connection")
		}
	}
}

func (s *controlStream) writeMessages() {
	for {
		select {
		case <-s.closeCh:
			s.logger.Info("close called, leaving control stream write loop")
			return
		case msg := <-s.sendQueue:
			s.logger.Info("sending control message", "type", fmt.Sprintf("%T", msg), "message", msg)
			buf := make([]byte, 0, 1500)
			buf = msg.Append(buf)
			if _, err := s.stream.Write(buf); err != nil {
				if err == io.EOF {
					s.logger.Info("write stream closed, leaving control stream write loop")
					return
				}
				s.logger.Error("failed to write to control stream", "error", err)
			}
		}
	}
}

func (s *controlStream) enqueue(m wire.Message) {
	select {
	case s.sendQueue <- m:
	default:
		s.logger.Warn("dropping control stream message because send queue is full")
	}
}

func (s *controlStream) close() {
	close(s.closeCh)
}
