package moqtransport

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire2"
	"github.com/mengelbart/moqtransport/varint"
)

var (
	errMissingPathParameter    = errors.New("missing path parameter")                       //nolint:unused
	errUnexpectedPathParameter = errors.New("unexpected path parameter on QUIC connection") //nolint:unused
)

type controlMessageReader interface {
	Read() (wire2.ControlMessage, error)
}

type controlMessageWriter interface {
	Write(wire2.ControlMessage) error
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

	version uint64
	path    string

	outgoingSubscribeRequestsLock           sync.RWMutex
	outgoingSubscribeRequests               map[uint64]*OutgoingSubscribeRequest
	outgoingSubscribeRequestsTrackAliasLock sync.RWMutex
	outgoingSubscribeRequestsTrackAlias     map[uint64]uint64
}

func NewSession(conn Connection, version uint64, path string, options ...Option) (*Session, error) {
	ctrlStream, err := conn.OpenUniStream()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctrlStreamAppender := wire2.NewAppender(ctrlStream, uint64(version))

	s := &Session{
		logger:                              defaultLogger.With("perspective", conn.Perspective()),
		ctx:                                 ctx,
		cancelCtx:                           cancel,
		conn:                                conn,
		requestIDs:                          newRequestIDGenerator(uint64(conn.Perspective()), 0 /*max*/, 2 /*step*/),
		remoteControlStream:                 nil,
		localControlStream:                  newLocalControlStream(ctrlStreamAppender),
		handler:                             nil,
		version:                             version,
		path:                                path,
		outgoingSubscribeRequests:           make(map[uint64]*OutgoingSubscribeRequest),
		outgoingSubscribeRequestsTrackAlias: make(map[uint64]uint64),
	}

	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	if err = s.localControlStream.write(&wire2.Setup{}); err != nil {
		// TODO: Close conn?
		return nil, err
	}
	s.logger.Debug("setup message sent", "version", version, "path", path)

	go s.readUniStreams()
	go s.readBidiStreams()
	go s.readDatagrams()

	return s, nil
}

func (s *Session) readUniStreams() {
	s.logger.Debug("starting to read uni streams")
	for {
		stream, err := s.conn.AcceptUniStream(s.ctx)
		if err != nil {
			// TODO
			panic(err)
		}
		go s.handleUniStream(stream)
	}
}

func (s *Session) handleUniStream(stream ReceiveStream) {
	s.logger.Debug("accepted new uni stream", "streamID", stream.StreamID())

	// TODO: This is a hacky way to figure out the stream type before
	// creating the parser. Ideally, we wouldn't need to know the stream
	// type, because we could parse all messages based on the message type
	// given in the first varint. However, the code points currently overlap
	// so it is impossible to distinguish between some messages that can
	// only be sent on different stream types.
	br := bufio.NewReader(stream)
	firstVarint, err := br.Peek(9)
	if err != nil {
		// TODO
		panic(err)
	}
	typ, _, err := varint.Parse(firstVarint)
	if err != nil {
		// TODO
		panic(err)
	}
	var streamType wire2.StreamType
	if typ == 0x2f00 {
		streamType = wire2.StreamTypeControl
	} else {
		streamType = wire2.StreamTypeData
	}
	s.logger.Debug("got stream type", "streamID", stream.StreamID(), "streamType", streamType)

	parser := wire2.NewParser(br, uint64(s.version), streamType)
	msg, err := parser.Read()
	if err != nil {
		s.logger.Error("error while reading message", "streamID", stream.StreamID(), "error", err, "typ", typ)
		// TODO
		panic(err)
	}
	switch m := msg.(type) {
	case *wire2.Setup:
		s.remoteControlStream = newRemoteControlStream(parser, m)
	case *wire2.SubgroupHeader:
		request, ok := s.getOutgoingSubscribeRequestByTrackAlias(m.TrackAlias)
		if ok {
			// TODO
			request.readStream(m, parser)
		}

	default:
		// TODO
		panic("unexpected message type")
	}
}

func (s *Session) readBidiStreams() {
	s.logger.Debug("starting to read bidi streams")
	for {
		stream, err := s.conn.AcceptStream(s.ctx)
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
			request := newIncomingSubscribeRequest(m, s.version, s.conn, wire2.NewAppender(stream, uint64(s.version)), parser)
			s.handler.HandleSubscribe(request)
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

func (s *Session) readDatagrams() {
	s.logger.Debug("starting to read datagrams")
	for {
		dgram, err := s.conn.ReceiveDatagram(s.ctx)
		if err != nil {
			// TODO
			panic(err)
		}
		msg := new(wire2.ObjectDatagram)
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

func (s *Session) receiveDatagram(msg *wire2.ObjectDatagram) error {
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

func (s *Session) setTrackAliasForRequest(requestID, trackAlias uint64) {
	s.outgoingSubscribeRequestsTrackAliasLock.Lock()
	defer s.outgoingSubscribeRequestsTrackAliasLock.Unlock()
	s.outgoingSubscribeRequestsTrackAlias[trackAlias] = requestID
}

func (s *Session) getOutgoingSubscribeRequestByTrackAlias(trackAlias uint64) (*OutgoingSubscribeRequest, bool) {
	s.outgoingSubscribeRequestsTrackAliasLock.RLock()
	defer s.outgoingSubscribeRequestsTrackAliasLock.RUnlock()
	requestID, ok := s.outgoingSubscribeRequestsTrackAlias[trackAlias]
	if !ok {
		return nil, false
	}
	s.outgoingSubscribeRequestsLock.RLock()
	defer s.outgoingSubscribeRequestsLock.RUnlock()
	request, ok := s.outgoingSubscribeRequests[requestID]
	return request, ok
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
	s.logger.Debug("opened new stream for subscribe request", "requestID", requestID, "namespace", namespace, "name", name)
	parser := wire2.NewParser(stream, uint64(s.version), wire2.StreamTypeRequest)
	appender := wire2.NewAppender(stream, uint64(s.version))

	request, err := newOutgoingSubscribeRequest(requestID, s, appender, parser, namespace, []byte(name))
	if err != nil {
		return nil, err
	}
	s.outgoingSubscribeRequestsLock.Lock()
	s.outgoingSubscribeRequests[requestID] = request
	s.outgoingSubscribeRequestsLock.Unlock()
	return request, nil
}

//nolint:unused
func (s *Session) onGoAway(msg *wire2.GoAway) {
	s.handler.HandleGoAway()
}
