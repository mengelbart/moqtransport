package moqtransport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/varint"
)

var (
	errMissingPathParameter    = errors.New("missing path parameter")                       //nolint:unused
	errUnexpectedPathParameter = errors.New("unexpected path parameter on WebTransport connection") //nolint:unused
)

type controlMessageReader interface {
	Read() (wire.ControlMessage, error)
}

type controlMessageWriter interface {
	Write(wire.ControlMessage) error
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
	wg        sync.WaitGroup

	closeLock sync.Mutex
	closeErr  error

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

func NewSession(conn Connection, path string, options ...Option) (*Session, error) {
	version := conn.ApplicationProtocol().versionNumber()
	if version == 0 {
		return nil, fmt.Errorf("unsupported application protocol: %q", conn.ApplicationProtocol())
	}
	logger := defaultLogger.With("perspective", conn.Perspective())
	logger.Debug("creating new session", "version", version, "path", path)

	ctrlStream, err := conn.OpenUniStream()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctrlStreamAppender := wire.NewAppender(ctrlStream, uint64(version))

	s := &Session{
		logger:                              logger,
		ctx:                                 ctx,
		cancelCtx:                           cancel,
		wg:                                  sync.WaitGroup{},
		conn:                                conn,
		requestIDs:                          newRequestIDGenerator(uint64(conn.Perspective())),
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

	// TODO: Write setup in a different goroutine to avoid blocking the session
	// constructor.
	if err = s.localControlStream.write(&wire.Setup{}); err != nil {
		// TODO: Close conn?
		return nil, err
	}
	s.logger.Debug("setup message sent", "version", version, "path", path)

	s.wg.Go(func() { s.readUniStreams() })
	s.wg.Go(func() { s.readBidiStreams() })
	s.wg.Go(func() { s.readDatagrams() })

	return s, nil
}

type SessionError struct {
	Code   uint64
	Reason string
	Remote bool
}

func (e *SessionError) Error() string {
	return e.Reason
}

func (e *SessionError) Is(target error) bool {
	other, ok := target.(*SessionError)
	return ok && e.Code == other.Code && e.Remote == other.Remote
}

func (s *Session) CloseWithError(code uint64, reason string) {
	s.closeWithError(&SessionError{Code: code, Reason: reason, Remote: false})
	s.wg.Wait()
}

func (s *Session) closeWithError(closeErr error) bool {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	if s.closeErr != nil {
		return false
	}
	s.closeErr = closeErr
	s.cancelCtx()

	code := uint64(ErrorCodeInternal)
	reason := ""
	if se, ok := closeErr.(*SessionError); ok {
		code = se.Code
		reason = se.Reason
	}
	_ = s.conn.CloseWithError(code, reason)

	return true
}

// goTracked runs f in a goroutine tracked by the session WaitGroup. It reports
// an error and does not start f if the session is already closed.
func (s *Session) goTracked(f func()) error {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	if s.closeErr != nil {
		return s.closeErr
	}
	s.wg.Go(f)
	return nil
}

// handleReaderError closes the session unless it is already shutting down, in
// which case the error is expected and ignored.
func (s *Session) handleReaderError(err error) {
	if s.ctx.Err() != nil {
		s.logger.Debug("ignoring reader error during session shutdown", "error", err)
		return
	}
	s.closeWithError(err)
}

func (s *Session) readUniStreams() {
	s.logger.Debug("starting to read uni streams")
	for {
		stream, err := s.conn.AcceptUniStream(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				s.logger.Debug("context canceled, stopping readUniStreams")
				return
			}
			s.closeWithError(err)
			return
		}
		s.wg.Go(func() { s.handleUniStream(stream) })
	}
}

func (s *Session) readBidiStreams() {
	s.logger.Debug("starting to read bidi streams")
	for {
		stream, err := s.conn.AcceptStream(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				s.logger.Debug("context canceled, stopping readBidiStreams")
				return
			}
			s.closeWithError(err)
			return
		}
		s.wg.Go(func() { s.handleBidiStream(stream) })
	}
}

func (s *Session) readDatagrams() {
	s.logger.Debug("starting to read datagrams")
	for {
		dgram, err := s.conn.ReceiveDatagram(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				s.logger.Debug("context canceled, stopping readDatagrams")
				return
			}
			s.closeWithError(err)
			return
		}
		msg := new(wire.ObjectDatagram)
		if _, err = msg.Parse(dgram); err != nil {
			s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("failed to parse datagram: %v", err), Remote: false})
			return
		}
		s.receiveDatagram(msg)
	}
}

func (s *Session) handleUniStream(stream ReceiveStream) {
	s.logger.Debug("accepted new uni stream", "streamID", stream.StreamID())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-s.ctx.Done():
			stream.Stop(0) // TODO: Set correct error code?
		}
	})

	// TODO: This is a hacky way to figure out the stream type before
	// creating the parser. Ideally, we wouldn't need to know the stream
	// type, because we could parse all messages based on the message type
	// given in the first varint. However, the code points currently overlap
	// so it is impossible to distinguish between some messages that can
	// only be sent on different stream types.
	br := bufio.NewReader(stream)
	firstVarint, err := peekFirstVarint(br)
	if err != nil {
		// Ignore stream
		return
	}
	typ, _, err := varint.Parse(firstVarint)
	if err != nil {
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("failed to parse first varint of stream: %v", err), Remote: false})
		return
	}
	var streamType wire.StreamType
	if typ == 0x2f00 {
		streamType = wire.StreamTypeControl
	} else {
		streamType = wire.StreamTypeData
	}
	s.logger.Debug("got stream type", "streamID", stream.StreamID(), "streamType", streamType)

	parser := wire.NewParser(br, uint64(s.version), streamType)
	msg, err := parser.Read()
	if err != nil {
		s.logger.Error("error while reading message", "streamID", stream.StreamID(), "error", err, "typ", typ)
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("failed to parse message: %v", err), Remote: false})
		return
	}
	switch m := msg.(type) {
	case *wire.Setup:
		s.remoteControlStream = newRemoteControlStream(m, parser, s)
		s.remoteControlStream.readMessages()
	case *wire.SubgroupHeader:
		request, ok := s.getOutgoingSubscribeRequestByTrackAlias(m.TrackAlias)
		if ok {
			// TODO
			request.readStream(m, parser)
		}

	default:
		// TODO
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("unexpected message type: %T", m), Remote: false})
		return
	}
}

func peekFirstVarint(br *bufio.Reader) ([]byte, error) {
	firstByte, err := br.Peek(1)
	if err != nil {
		return nil, err
	}

	needed := 1
	for i := 7; i >= 0; i-- {
		if (firstByte[0] & (1 << uint(i))) == 0 {
			break
		}
		needed++
	}

	return br.Peek(needed)
}

func (s *Session) handleBidiStream(stream Stream) {
	s.logger.Debug("accepted new bidi stream", "streamID", stream.StreamID())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-s.ctx.Done():
			stream.Stop(0)  // TODO: Set correct error code?
			stream.Reset(0) // TODO: Set correct error code?
		}
	})

	parser := wire.NewParser(stream, uint64(s.version), wire.StreamTypeRequest)
	msg, err := parser.Read()
	if err != nil {
		// Ignore stream
		return
	}
	switch m := msg.(type) {
	case *wire.TrackStatus:
	case *wire.Subscribe:
		// TODO: Handle incoming request
		if s.handler == nil {
			return
		}
		request := newIncomingSubscribeRequest(m, s, wire.NewAppender(stream, uint64(s.version)), parser)
		s.handler.HandleSubscribe(request)
		request.readMessages()
	case *wire.Publish:
	case *wire.Fetch:
	case *wire.PublishNamespace:
	case *wire.SubscribeNamespace:
	case *wire.SubscribeTracks:
	default:
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("unexpected message type: %T", m), Remote: false})
		return
	}
}

func (s *Session) receiveDatagram(msg *wire.ObjectDatagram) {
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
	s.closeLock.Lock()
	if s.closeErr != nil {
		s.closeLock.Unlock()
		return nil, s.closeErr
	}
	s.closeLock.Unlock()

	requestID := s.requestIDs.next()
	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.Debug("opened new stream for subscribe request", "requestID", requestID, "namespace", namespace, "name", name)
	parser := wire.NewParser(stream, uint64(s.version), wire.StreamTypeRequest)
	appender := wire.NewAppender(stream, uint64(s.version))

	request, err := newOutgoingSubscribeRequest(requestID, s, appender, parser, namespace, []byte(name))
	if err != nil {
		return nil, err
	}
	s.outgoingSubscribeRequestsLock.Lock()
	s.outgoingSubscribeRequests[requestID] = request
	s.outgoingSubscribeRequestsLock.Unlock()

	if err := s.goTracked(request.readMessages); err != nil {
		return nil, err
	}
	return request, nil
}

func (s *Session) onGoAway(msg *wire.GoAwayCtrl) {
	if s.handler == nil {
		return
	}
	s.handler.HandleGoAway(msg.NewSessionURI)
}
