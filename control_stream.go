package moqtransport

import (
	"context"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
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

type controlStream struct {
	logger    *slog.Logger
	qlogger   *qlog.Logger
	ctx       context.Context
	cancelCtx context.CancelCauseFunc
	queue     chan wire.ControlMessage
	transport *Transport
}

func newControlStream(t *Transport, qlogger *qlog.Logger) *controlStream {
	ctx, cancel := context.WithCancelCause(context.Background())
	cs := &controlStream{
		logger:    nil,
		qlogger:   qlogger,
		ctx:       ctx,
		cancelCtx: cancel,
		queue:     make(chan wire.ControlMessage, 100),
		transport: t,
	}
	return cs
}

func (s *controlStream) accept(conn Connection, receiver controlMessageReceiver) error {
	s.logger = defaultLogger.With("perspective", conn.Perspective())
	stream, err := conn.AcceptStream(s.ctx)
	if err != nil {
		return err
	}
	if s.qlogger != nil {
		s.qlogger.Log(moqt.StreamTypeSetEvent{
			Owner:      moqt.GetOwner(moqt.OwnerRemote),
			StreamID:   stream.StreamID(),
			StreamType: "control",
		})
	}

	go s.sendLoop(stream)
	go s.receiveLoop(newControlMessageParser(stream), receiver)
	return nil
}

func (s *controlStream) open(conn Connection, receiver controlMessageReceiver) error {
	s.logger = defaultLogger.With("perspective", conn.Perspective())
	stream, err := conn.OpenStreamSync(s.ctx)
	if err != nil {
		return err
	}
	if s.qlogger != nil {
		s.qlogger.Log(moqt.StreamTypeSetEvent{
			Owner:      moqt.GetOwner(moqt.OwnerLocal),
			StreamID:   stream.StreamID(),
			StreamType: "control",
		})
	}
	go s.sendLoop(stream)
	go s.receiveLoop(newControlMessageParser(stream), receiver)
	return nil
}

// queueControlMessage implements controlMessageSender.
func (s *controlStream) queueControlMessage(msg wire.ControlMessage) error {
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case s.queue <- msg:
		return nil
	default:
		return errControlMessageQueueOverflow
	}
}

func (s *controlStream) sendLoop(writer SendStream) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.queue:
			buf, err := compileMessage(msg)
			if err != nil {
				s.logger.Error("control message dropped", "error", err)
				continue
			}
			if s.qlogger != nil {
				s.qlogger.Log(moqt.ControlMessageEvent{
					EventName: moqt.ControlMessageEventCreated,
					StreamID:  writer.StreamID(),
					Length:    uint64(len(buf)),
					Message:   msg,
				})
			}
			s.logger.Info("sending message", "type", msg.Type().String(), "msg", msg)
			_, err = writer.Write(buf)
			if err != nil {
				s.logger.Error("failed to write control message", "error", err)
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
			s.logger.Error("failed to parse control message", "error", err)
			s.close(err)
			return
		}
		if s.qlogger != nil {
			s.qlogger.Log(moqt.ControlMessageEvent{
				EventName: moqt.ControlMessageEventParsed,
				StreamID:  0,
				Length:    0,
				Message:   msg,
			})
		}
		if err = receiver.receive(msg); err != nil {
			s.logger.Error("session failed to handle control message", "error", err)
			s.close(err)
			return
		}
	}
}

func (s *controlStream) close(err error) {
	s.cancelCtx(err)
	s.transport.handleProtocolViolation(err)
}
