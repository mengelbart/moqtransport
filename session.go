package moqtransport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
	"golang.org/x/exp/slices"
)

type parser interface {
	parse() (message, error)
}

type parserFactory interface {
	new(messageReader) parser
}

type parserFactoryFn func(messageReader) parser

func (f parserFactoryFn) new(r messageReader) parser {
	return f(r)
}

type Session struct {
	conn       connection
	ctrlStream stream
	mr         *messageRouter

	parserFactory parserFactory

	logger *slog.Logger
}

func newClientSession(ctx context.Context, conn connection, clientRole Role, enableDatagrams bool) (*Session, error) {
	pf := newParserFactory()
	ctrlStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening control stream failed: %w", err)
	}
	csm := &clientSetupMessage{
		SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: uint64(clientRole),
			},
		},
	}
	if err = sendOnStream(ctrlStream, csm); err != nil {
		return nil, fmt.Errorf("sending message on control stream failed: %w", err)
	}
	msgParser := pf.new(quicvarint.NewReader(ctrlStream))
	msg, err := msgParser.parse()
	if err != nil {
		return nil, fmt.Errorf("parsing message filed: %w", err)
	}
	ssm, ok := msg.(*serverSetupMessage)
	if !ok {
		return nil, &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected first message on control stream",
		}
	}
	if !slices.Contains(csm.SupportedVersions, ssm.SelectedVersion) {
		return nil, errUnsupportedVersion
	}
	return newSession(ctx, conn, ctrlStream, pf, enableDatagrams), nil
}

func newServerSession(ctx context.Context, conn connection, enableDatagrams bool) (*Session, error) {
	pf := newParserFactory()
	ctrlStream, err := conn.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("accepting control stream failed: %w", err)
	}
	p := pf.new(quicvarint.NewReader(ctrlStream))
	m, err := p.parse()
	if err != nil {
		return nil, fmt.Errorf("parsing message filed: %w", err)
	}
	msg, ok := m.(*clientSetupMessage)
	if !ok {
		return nil, &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected first message on control stream",
		}
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, DRAFT_IETF_MOQ_TRANSPORT_01) {
		return nil, errUnsupportedVersion
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		return nil, &moqError{
			code:    genericErrorErrorCode,
			message: "missing role parameter",
		}
	}
	// TODO: save role parameter
	ssm := &serverSetupMessage{
		SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
		SetupParameters: map[uint64]parameter{},
	}
	if err := sendOnStream(ctrlStream, ssm); err != nil {
		return nil, fmt.Errorf("sending message on control stream failed: %w", err)
	}
	return newSession(ctx, conn, ctrlStream, pf, enableDatagrams), nil
}

func newSession(ctx context.Context, conn connection, ctrlStream stream, pf parserFactory, enableDatagrams bool) *Session {
	s := &Session{
		conn:          conn,
		ctrlStream:    ctrlStream,
		mr:            nil,
		parserFactory: pf,
		logger:        defaultLogger.With(componentKey, "MOQ_SESSION"),
	}
	mr := newMessageRouter(conn, s)
	s.mr = mr
	go s.acceptUnidirectionalStreams(ctx)
	if enableDatagrams {
		go s.acceptDatagrams(ctx)
	}
	go s.readMessages(quicvarint.NewReader(ctrlStream), s.mr.handleControlMessage)
	return s
}

func sendOnStream(stream sendStream, msg message) error {
	buf := make([]byte, 0, 1500)
	buf = msg.append(buf)
	if _, err := stream.Write(buf); err != nil {
		return err
	}
	return nil
}

func (s *Session) send(msg message) error {
	return sendOnStream(s.ctrlStream, msg)
}

func (s *Session) acceptUnidirectionalStreams(ctx context.Context) {
	for {
		stream, err := s.conn.AcceptUniStream(ctx)
		if err != nil {
			s.logger.Error("failed to accept uni stream", "error", err)
			return
		}
		go s.readMessages(quicvarint.NewReader(stream), s.mr.handleObjectMessage)
	}
}

func (s *Session) acceptDatagrams(ctx context.Context) {
	for {
		dgram, err := s.conn.ReceiveMessage(ctx)
		if err != nil {
			s.logger.Error("failed to receive datagram", "error", err)
			return
		}
		go s.readMessages(bytes.NewReader(dgram), s.mr.handleObjectMessage)
	}
}

type messageHandler func(message) error

func (s *Session) readMessages(r messageReader, handle messageHandler) {
	msgParser := s.parserFactory.new(r)
	for {
		msg, err := msgParser.parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			s.logger.Error("TODO", "error", err)
			return
		}
		if err = handle(msg); err != nil {
			panic(fmt.Sprintf("TODO: %v", err))
		}
	}
}

func (s *Session) ReadSubscription(ctx context.Context) (*SendSubscription, error) {
	return s.mr.readSubscription(ctx)
}

func (s *Session) ReadAnnouncement(ctx context.Context) (*Announcement, error) {
	return s.mr.readAnnouncement(ctx)
}

func (s *Session) Subscribe(ctx context.Context, subscribeID, trackAlias uint64, namespace, trackname, auth string) (*ReceiveSubscription, error) {
	return s.mr.subscribe(ctx, subscribeID, trackAlias, namespace, trackname, auth)
}

func (s *Session) Announce(ctx context.Context, namespace string) error {
	return s.mr.announce(ctx, namespace)
}

func (s *Session) CloseWithError(code uint64, msg string) {

}
