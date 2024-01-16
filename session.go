package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
	"golang.org/x/exp/slices"
)

var (
	errUnexpectedMessage     = errors.New("got unexpected message")
	errInvalidTrackNamespace = errors.New("got invalid tracknamespace")
	errUnknownTrack          = errors.New("received object for unknown track")
	errUnsupportedVersion    = errors.New("unsupported version")
	errMissingRoleParameter  = errors.New("missing role parameter")
	errInternal              = errors.New("internal error occured")
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

type messageKey struct {
	mt messageType
	id string
}

type keyer interface {
	key() messageKey
}

type keyedMessage interface {
	message
	keyer
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
		return nil, err
	}
	msgParser := pf.new(quicvarint.NewReader(ctrlStream))
	msg, err := msgParser.parse()
	if err != nil {
		return nil, err
	}
	ssm, ok := msg.(*serverSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
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
		return nil, err
	}
	p := pf.new(quicvarint.NewReader(ctrlStream))
	m, err := p.parse()
	if err != nil {
		return nil, err
	}
	msg, ok := m.(*clientSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, DRAFT_IETF_MOQ_TRANSPORT_01) {
		return nil, errUnsupportedVersion
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		return nil, errMissingRoleParameter
	}
	// TODO: save role parameter
	ssm := &serverSetupMessage{
		SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
		SetupParameters: map[uint64]parameter{},
	}
	if err := sendOnStream(ctrlStream, ssm); err != nil {
		return nil, err
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
	go s.readMessages(quicvarint.NewReader(ctrlStream))
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
		go s.readMessages(quicvarint.NewReader(stream))
	}
}

func (s *Session) acceptDatagrams(ctx context.Context) {
	for {
		dgram, err := s.conn.ReceiveMessage(ctx)
		if err != nil {
			s.logger.Error("failed to receive datagram", "error", err)
			return
		}
		go s.readMessages(bytes.NewReader(dgram))
	}
}

func (s *Session) readMessages(r messageReader) {
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
		if err = s.mr.handleMessage(msg); err != nil {
			panic(fmt.Sprintf("TODO: %v", err))
		}

	}
}

func (s *Session) ReadSubscription(ctx context.Context) (*Subscription, error) {
	return s.mr.readSubscription(ctx)
}

func (s *Session) ReadAnnouncement(ctx context.Context) (*Announcement, error) {
	return s.mr.readAnnouncement(ctx)
}

func (s *Session) Subscribe(ctx context.Context, namespace, trackname, auth string) (*ReceiveTrack, error) {
	return s.mr.subscribe(ctx, namespace, trackname, auth)
}

func (s *Session) Announce(ctx context.Context, namespace string) error {
	return s.mr.announce(ctx, namespace)
}

func (s *Session) CloseWithError(code uint64, msg string) {

}
