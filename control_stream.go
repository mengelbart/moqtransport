package moqtransport

import (
	"context"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type controlMessageReceiver interface {
	receive(wire.ControlMessage) error
}

type controlMessageParser interface {
	Parse() (wire.ControlMessage, error)
}

func newControlMessageParser(r io.Reader) controlMessageParser {
	return wire.NewControlMessageParser(r)
}

type controlMessageSender interface {
	QueueControlMessage(wire.ControlMessage) error
	close(err error)
}

type controlStream struct {
	logger    *slog.Logger
	ctx       context.Context
	cancelCtx context.CancelCauseFunc
	queue     chan wire.ControlMessage
	transport *Transport
}

func newControlStream(t *Transport) *controlStream {
	ctx, cancel := context.WithCancelCause(context.Background())
	cs := &controlStream{
		ctx:       ctx,
		cancelCtx: cancel,
		queue:     make(chan wire.ControlMessage, 100),
		transport: t,
	}
	return cs
}

func (s *controlStream) accept(conn Connection, receiver controlMessageReceiver) {
	stream, err := conn.AcceptStream(s.ctx)
	if err != nil {
		s.close(err)
		return
	}
	s.logger = defaultLogger.With("perspective", conn.Perspective())
	go s.sendLoop(stream)
	go s.receiveLoop(newControlMessageParser(stream), receiver)
}

func (s *controlStream) open(conn Connection, receiver controlMessageReceiver) {
	stream, err := conn.OpenStreamSync(s.ctx)
	if err != nil {
		s.transport.handleProtocolViolation(err)
		return
	}
	s.logger = defaultLogger.With("perspective", conn.Perspective())
	go s.sendLoop(stream)
	go s.receiveLoop(newControlMessageParser(stream), receiver)
}

// queueControlMessage implements controlMessageSender.
func (s *controlStream) QueueControlMessage(msg wire.ControlMessage) error {
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case s.queue <- msg:
		return nil
	default:
		return ErrControlMessageQueueOverflow
	}
}

func (s *controlStream) sendLoop(writer io.Writer) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.queue:
			s.logger.Info("sending message", "type", msg.Type().String(), "msg", msg)
			_, err := writer.Write(compileMessage(msg))
			if err != nil {
				s.close(err)
				return
			}
		}
	}
}

func (s *controlStream) receiveLoop(parser controlMessageParser, receiver controlMessageReceiver) {
	for {
		msg, err := parser.Parse()
		if err != nil {
			s.close(err)
			return
		}
		if err = receiver.receive(msg); err != nil {
			s.close(err)
			return
		}
	}
}

func (s *controlStream) close(err error) {
	s.cancelCtx(err)
	s.transport.handleProtocolViolation(err)
}
