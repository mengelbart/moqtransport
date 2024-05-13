package moqtransport

import (
	"io"
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type controlStream struct {
	logger    *slog.Logger
	stream    Stream
	handle    messageHandler
	parser    parser
	sendQueue chan message
	closeCh   chan struct{}
}

type messageHandler func(message) error

func newControlStream(s Stream, h messageHandler) *controlStream {
	cs := &controlStream{
		logger:    defaultLogger.WithGroup("MOQ_CONTROL_STREAM"),
		stream:    s,
		handle:    h,
		parser:    newParser(quicvarint.NewReader(s)),
		sendQueue: make(chan message, 64),
		closeCh:   make(chan struct{}),
	}
	go cs.readMessages()
	go cs.writeMessages()
	return cs
}

func (s *controlStream) readMessages() {
	for {
		msg, err := s.parser.parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			s.logger.Error("TODO", "error", err)
			return
		}
		if err = s.handle(msg); err != nil {
			s.logger.Error("failed to handle control stream message", "error", err)
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
			s.logger.Info("sending control message", "message", msg)
			buf := make([]byte, 0, 1500)
			buf = msg.append(buf)
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

func (s *controlStream) enqueue(m message) {
	select {
	case s.sendQueue <- m:
	default:
		s.logger.Warn("dropping control stream message because send queue is full")
	}
}

func (s *controlStream) close() {
	close(s.closeCh)
}
