package moqtransport

import (
	"bufio"
	"context"
	"errors"
	"iter"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/internal/wire2"
	"github.com/mengelbart/moqtransport/varint"
)

var (
	errMaxRequestIDViolated             = errors.New("max request ID violated")
	errClientReceivedClientSetup        = errors.New("client received client setup message")
	errServerReceveidServerSetup        = errors.New("server received server setup message")
	errIncompatibleVersions             = errors.New("incompatible versions")
	errUnexpectedMessageType            = errors.New("unexpected message type")
	errUnexpectedMessageTypeBeforeSetup = errors.New("unexpected message type before setup")
	errUnknownTrackAlias                = errors.New("unknown track alias")
	errMissingPathParameter             = errors.New("missing path parameter")
	errUnexpectedPathParameter          = errors.New("unexpected path parameter on QUIC connection")
	errUnknownTrackStatusRequest        = errors.New("got unexpected track status requrest")
	errUnknownRequestID                 = errors.New("unknown request ID")
)

type controlMessageReader interface {
	Read() (wire2.ControlMessage, error)
}

type controlMessageWriter interface {
	Write(wire2.ControlMessage) error
}

type objectMessageParser interface {
	Type() wire.StreamType
	Identifier() uint64
	Messages() iter.Seq2[*wire.ObjectMessage, error]
}

type Option func(*Session) error

func WithHandler(handler Handler) Option {
	return func(s *Session) error {
		s.handler = handler
		return nil
	}
}

// A Session is an endpoint of a MoQ Session session.
type Session struct {
	logger *slog.Logger

	ctx       context.Context
	cancelCtx context.CancelFunc

	conn       Connection
	requestIDs *requestIDGenerator

	remoteControlStream *remoteControlStream
	localControlStream  *localControlStream

	handler Handler

	version wire.Version
	path    string
}

func NewSession(conn Connection, version wire.Version, path string, options ...Option) (*Session, error) {
	ctx, cancel := context.WithCancel(context.Background())

	ctrlStream, err := conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	ctrlStreamAppender := wire2.NewAppender(ctrlStream, uint64(version))

	s := &Session{
		logger:              defaultLogger.With("perspective", conn.Perspective()),
		ctx:                 ctx,
		cancelCtx:           cancel,
		conn:                conn,
		requestIDs:          newRequestIDGenerator(uint64(conn.Perspective()), 0 /*max*/, 2 /*step*/),
		remoteControlStream: nil,
		localControlStream:  newLocalControlStream(ctrlStreamAppender),
		handler:             nil,
		version:             version,
		path:                path,
	}

	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	s.localControlStream.write(&wire2.Setup{})

	go s.readUniStreams(s.ctx)
	go s.readBidiStreams(s.ctx)
	go s.readDatagrams(s.ctx)

	return s, nil
}

func (s *Session) readUniStreams(ctx context.Context) {
	for {
		stream, err := s.conn.AcceptUniStream(ctx)
		if err != nil {
			// TODO
			panic(err)
		}

		// TODO: This is a hacky way to figure out the stream type before
		// creating the parser. Ideally, we wouldn't need to know the stream
		// type, because we could parse all messages based on the message type
		// given in the first varint. However, the code points currently overlap
		// so it is impossible to distinguish between some messages that can
		// only be sent on different stream types.
		br := bufio.NewReader(stream)
		firstVarint, err := br.Peek(9)
		typ, _, err := varint.Parse(firstVarint)
		var streamType wire2.StreamType
		if typ == 0x2f00 {
			streamType = wire2.StreamTypeControl
		} else {
			streamType = wire2.StreamTypeData
		}

		parser := wire2.NewParser(stream, uint64(s.version), streamType)
		msg, err := parser.Read()
		if err != nil {
			// TODO
			panic(err)
		}
		switch m := msg.(type) {
		case *wire2.Setup:
			s.remoteControlStream = newRemoteControlStream(parser, m)
		default:
			// TODO
			panic("unexpected message type")
		}
	}
}

func (s *Session) readBidiStreams(ctx context.Context) error {
	for {
		stream, err := s.conn.AcceptStream(ctx)
		if err != nil {
			// TODO: Handle error
			panic(err)
		}

		// TODO: The following should happen in a different goroutine so we
		// don't block new requests by waiting for the remaining bytes of the
		// first message of this request.
		parser := wire2.NewParser(stream, uint64(s.version), wire2.StreamTypeRequest)
		msg, err := parser.Read()
		if err != nil {
			// TODO: Handle error
			panic(err)
		}
		switch m := msg.(type) {
		case *wire2.TrackStatus:
		case *wire2.Subscribe:
			// TODO: Handle incoming request
			newIncomingSubscribeRequest(m, s, wire2.NewAppender(stream, uint64(s.version)), parser)
		case *wire2.Publish:
		case *wire2.Fetch:
		case *wire2.PublishNamespace:
		case *wire2.SubscribeNamespace:
		case *wire2.SubscribeTracks:
		default:
			// TODO: Handle error
			panic("unexpected message type")
		}
	}
}

func (s *Session) readDatagrams(ctx context.Context) {
	for {
		dgram, err := s.conn.ReceiveDatagram(ctx)
		if err != nil {
			// TODO
			panic(err)
		}
		msg := new(wire.ObjectDatagramMessage)
		if _, err = msg.Parse(dgram); err != nil {
			// TODO
			panic(err)
		}
		if err := s.receiveDatagram(msg); err != nil {
			// TODO
			panic(err)
		}
	}
}

func (s *Session) handleUniStream(parser objectMessageParser) error {
	if parser.Type() == wire.StreamTypeFetch {
		return s.readFetchStream(parser)
	}
	return s.readSubgroupStream(parser)
}

func (s *Session) readFetchStream(parser objectMessageParser) error {
	// TODO: Implement routing to correct subscribe request or buffer until track alias arrives
	// rt, ok := s.remoteTrackByRequestID(parser.Identifier())
	// if !ok {
	// 	return errUnknownRequestID
	// }
	// return rt.readFetchStream(parser)
	return nil
}

func (s *Session) readSubgroupStream(parser objectMessageParser) error {
	// TODO: Implement routing to correct subscribe request or buffer until track alias arrives
	// rt, ok := s.remoteTrackByTrackAlias(parser.Identifier())
	// if !ok {
	// 	return errUnknownRequestID
	// }
	// return rt.readSubgroupStream(parser)
	return nil
}

func (s *Session) receiveDatagram(msg *wire.ObjectDatagramMessage) error {
	// TODO: Implement routing to correct subscribe request or buffer until track alias arrives
	// subscription, ok := s.remoteTrackByTrackAlias(msg.TrackAlias)
	// if !ok {
	// 	return errUnknownTrackAlias
	// }
	// subscription.push(&Object{
	// 	GroupID:              msg.GroupID,
	// 	ObjectID:             msg.ObjectID,
	// 	ForwardingPreference: ObjectForwardingPreferenceDatagram,
	// 	Payload:              msg.ObjectPayload,
	// })
	return nil
}

func (s *Session) Subscribe(
	ctx context.Context,
	namespace [][]byte,
	name string,
) (*OutgoingSubscribeRequest, error) {
	requestID := s.requestIDs.next()
	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	parser := wire2.NewParser(stream, uint64(s.version), wire2.StreamTypeRequest)
	appender := wire2.NewAppender(stream, uint64(s.version))

	_, err = newOutgoingSubscribeRequest(requestID, s, appender, parser, namespace, []byte(name))

	return nil, nil
}

func (s *Session) onGoAway(msg *wire.GoAwayMessage) {
	s.handler.HandleGoAway()
}
