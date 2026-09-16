package moqtransport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/quic"
	"github.com/mengelbart/moqtransport/varint"
)

var (
	errMissingPathParameter    = errors.New("missing path parameter")
	errUnexpectedPathParameter = errors.New("unexpected path parameter on WebTransport connection")
)

const (
	defaultMaxPendingObjects   = 100
	defaultMaxPendingTracks    = 16
	defaultSubscribeBufferSize = 100
	defaultPublishDoneTimeout  = 5 * time.Second

	defaultPublisherPriority uint8 = 128

	maxGoAwayURILength = 8192
)

type messageReader interface {
	Read() (wire.ControlMessage, error)
}

type messageWriter interface {
	Write(wire.ControlMessage) error
}

type streamCanceller interface {
	Reset(uint32)
	Stop(uint32)
}

type requestStream struct {
	messageReader
	messageWriter
	io.Closer
	streamCanceller
}

func newRequestStream(stream quic.Stream, version uint64) (*requestStream, error) {
	parser, err := wire.NewParser(stream, version, wire.StreamTypeRequest)
	if err != nil {
		return nil, err
	}
	return &requestStream{parser, wire.NewAppender(stream, version), stream, stream}, nil
}

func (s *requestStream) cancel(code StreamResetErrorCode) {
	s.Reset(uint32(code))
	s.Stop(uint32(code))
}

type Option func(*Session) error

func WithHandler(handler Handler) Option {
	return func(s *Session) error {
		s.handler = handler
		return nil
	}
}

// WithMaxPendingObjects sets the number of datagram objects buffered per track
// alias that is not bound to a receiver yet. Objects over the limit are
// dropped.
func WithMaxPendingObjects(n int) Option {
	return func(s *Session) error {
		if n <= 0 {
			return fmt.Errorf("max pending objects must be greater than zero: %v", n)
		}
		s.maxPendingObjects = n
		return nil
	}
}

// WithMaxPendingTracks sets the number of unbound track aliases that may buffer
// objects or hold a blocked data stream at the same time. Exceeding it ends the
// session.
func WithMaxPendingTracks(n int) Option {
	return func(s *Session) error {
		if n <= 0 {
			return fmt.Errorf("max pending tracks must be greater than zero: %v", n)
		}
		s.maxPendingTracks = n
		return nil
	}
}

// WithSubscribeBufferSize sets the number of objects buffered in a
// subscription.
func WithSubscribeBufferSize(n int) Option {
	return func(s *Session) error {
		if n <= 0 {
			return fmt.Errorf("subscribe buffer size must be greater than zero: %v", n)
		}
		s.subscribeBufferSize = n
		return nil
	}
}

// WithPublishDoneTimeout sets how long a subscription waits for late data
// streams after PUBLISH_DONE before its state is dropped. The wait ends early
// once every stream announced in PUBLISH_DONE has ended.
func WithPublishDoneTimeout(d time.Duration) Option {
	return func(s *Session) error {
		if d <= 0 {
			return fmt.Errorf("publish done timeout must be greater than zero: %v", d)
		}
		s.publishDoneTimeout = d
		return nil
	}
}

// A Session is an endpoint of a MoQ Session session.
type Session struct {
	logger *slog.Logger

	ctx       context.Context
	cancelCtx context.CancelCauseFunc
	wg        sync.WaitGroup

	closeLock sync.Mutex
	closeErr  error

	conn       quic.Connection
	requestIDs *requestIDGenerator

	peerRequestIDsLock   sync.Mutex
	peerRequestIDs       map[uint64]struct{}
	largestPeerRequestID uint64

	goAwaySent atomic.Bool

	controlStreamLock   sync.Mutex
	remoteControlStream *remoteControlStream
	localControlStream  *localControlStream
	remotePath          string

	handler Handler

	version uint64
	path    string

	tracksLock    sync.Mutex
	tracks        map[uint64]*trackEntry
	pendingTracks int

	maxPendingObjects   int
	maxPendingTracks    int
	subscribeBufferSize int
	publishDoneTimeout  time.Duration
}

// NewSession creates a session on conn. It never closes conn: if NewSession
// returns an error, nothing has been written and the caller keeps ownership of
// conn. Once a session exists, only the session closes conn. Errors that occur
// after NewSession returned, including a failed SETUP write, are reported
// through Context.
func NewSession(conn quic.Connection, path string, options ...Option) (*Session, error) {
	version := conn.ApplicationProtocol().VersionNumber()
	if version == 0 {
		return nil, fmt.Errorf("unsupported application protocol: %q", conn.ApplicationProtocol())
	}
	logger := defaultLogger.With("perspective", conn.Perspective())
	logger.Debug("creating new session", "version", version, "path", path)

	s := &Session{
		logger:              logger,
		wg:                  sync.WaitGroup{},
		conn:                conn,
		requestIDs:          newRequestIDGenerator(uint64(conn.Perspective())),
		peerRequestIDs:      make(map[uint64]struct{}),
		remoteControlStream: nil,
		localControlStream:  nil,
		handler:             nil,
		version:             version,
		path:                path,
		tracks:              make(map[uint64]*trackEntry),
		maxPendingObjects:   defaultMaxPendingObjects,
		maxPendingTracks:    defaultMaxPendingTracks,
		subscribeBufferSize: defaultSubscribeBufferSize,
		publishDoneTimeout:  defaultPublishDoneTimeout,
	}

	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	ctrlStream, err := conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	s.localControlStream = newLocalControlStream(wire.NewAppender(ctrlStream, uint64(version)))

	s.ctx, s.cancelCtx = context.WithCancelCause(context.Background())

	s.wg.Go(func() { s.sendSetup() })
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

// Context returns a context that is canceled when the session closes.
// context.Cause reports the error that closed it.
func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) sendSetup() {
	setup := &wire.Setup{}
	if s.conn.Protocol() == quic.ProtocolQUIC {
		setup.Options = []wire.KeyValuePair{
			{Type: wire.SetupOptionTypePath, Bytes: []byte(s.path)},
		}
	}
	if err := s.localControlStream.writeSetup(setup); err != nil {
		s.closeOnError(err)
		return
	}
	s.logger.Debug("setup message sent", "version", s.version, "path", s.path)
}

func (s *Session) closeWithError(closeErr error) bool {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	if s.closeErr != nil {
		return false
	}
	s.closeErr = closeErr
	s.cancelCtx(closeErr)

	code := uint64(ErrorCodeInternal)
	reason := ""
	if se, ok := closeErr.(*SessionError); ok {
		code = se.Code
		reason = se.Reason
	}
	_ = s.conn.CloseWithError(code, reason)

	return true
}

// requestErrorFromWire converts a received REQUEST_ERROR and validates its
// redirect. namespaceScoped is true for requests that carry no track name. A
// non-nil SessionError is a PROTOCOL_VIOLATION the caller must close with.
func (s *Session) requestErrorFromWire(msg *wire.RequestError, namespaceScoped bool) (*RequestError, *SessionError) {
	err := &RequestError{
		Code:          RequestErrorCode(msg.ErrorCode),
		Reason:        msg.ErrorReason,
		RetryInterval: msg.RetryInterval,
	}
	if err.Code != RequestErrorCodeRedirect {
		return err, nil
	}
	if s.conn.Perspective() == quic.PerspectiveServer && msg.Redirect.ConnectURI != "" {
		return nil, &SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: "redirect with connect URI received by server",
		}
	}
	if namespaceScoped && len(msg.Redirect.TrackName) > 0 {
		return nil, &SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: "redirect with track name for namespace request",
		}
	}
	err.Redirect = &Redirect{
		ConnectURI: msg.Redirect.ConnectURI,
		Namespace:  msg.Redirect.TrackNamespace,
		Name:       msg.Redirect.TrackName,
	}
	return err, nil
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

// closeOnError closes the session unless it is already shutting down, in
// which case the error is expected and ignored.
func (s *Session) closeOnError(err error) {
	if s.ctx.Err() != nil {
		s.logger.Debug("ignoring error during session shutdown", "error", err)
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
		msg := new(wire.DatagramObject)
		if err = msg.Parse(dgram); err != nil {
			s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("failed to parse datagram: %v", err), Remote: false})
			return
		}
		s.receiveDatagram(msg)
	}
}

func (s *Session) handleUniStream(stream quic.ReceiveStream) {
	s.logger.Debug("accepted new uni stream", "streamID", stream.StreamID())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-s.ctx.Done():
			stream.Stop(uint32(StreamResetErrorCodeSessionClosed))
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
		s.logger.Error("failed to peek first varint of stream", "streamID", stream.StreamID(), "error", err)
		stream.Stop(uint32(StreamResetErrorCodeInternal))
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

	parser, err := wire.NewParser(br, uint64(s.version), streamType)
	if err != nil {
		s.logger.Error("failed to create parser", "streamID", stream.StreamID(), "error", err)
		stream.Stop(uint32(StreamResetErrorCodeInternal))
		return
	}
	msg, err := parser.Read()
	if err != nil {
		s.logger.Error("error while reading message", "streamID", stream.StreamID(), "error", err, "typ", typ)
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("failed to parse message: %v", err), Remote: false})
		return
	}
	switch m := msg.(type) {
	case *wire.Setup:
		path, err := validatePathParameter(m.Options, s.conn.Protocol() == quic.ProtocolQUIC)
		if err != nil {
			s.closeWithError(&SessionError{Code: uint64(ErrorCodeInvalidPath), Reason: err.Error(), Remote: false})
			return
		}
		rcs := newRemoteControlStream(m, parser, s)
		if !s.setRemoteControlStream(rcs, path) {
			s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: "duplicate control stream", Remote: false})
			return
		}
		rcs.readMessages()
	case *wire.SubgroupHeader:
		receiver, err := s.waitForReceiver(m.TrackAlias)
		if err != nil {
			return
		}
		ds := newSubgroupStream(stream, receiver, s)
		receiver.addSubgroupStream(ds)
		defer receiver.removeSubgroupStream(ds)
		ds.read(m, parser)
	case *wire.Padding:
		if _, err := io.Copy(io.Discard, br); err != nil {
			s.logger.Debug("error while discarding padding stream", "streamID", stream.StreamID(), "error", err)
		}
	case *wire.FetchHeader:
		if _, err := io.Copy(io.Discard, br); err != nil {
			s.logger.Debug("error while discarding fetch stream", "streamID", stream.StreamID(), "error", err)
		}
	default:
		// TODO
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("unexpected message type: %T", m), Remote: false})
		return
	}
}

func (s *Session) setRemoteControlStream(rcs *remoteControlStream, path string) bool {
	s.controlStreamLock.Lock()
	defer s.controlStreamLock.Unlock()
	if s.remoteControlStream != nil {
		return false
	}
	s.remoteControlStream = rcs
	s.remotePath = path
	return true
}

// Path returns the path the peer sent in its SETUP message. It is only
// populated for QUIC connections, since WebTransport conveys the path in the
// HTTP request instead. It is empty until the peer's SETUP has been received.
func (s *Session) Path() string {
	s.controlStreamLock.Lock()
	defer s.controlStreamLock.Unlock()
	return s.remotePath
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

func requestIDOfMessage(msg wire.ControlMessage) (uint64, bool) {
	switch m := msg.(type) {
	case *wire.Subscribe:
		return m.RequestID, true
	case *wire.Publish:
		return m.RequestID, true
	case *wire.Fetch:
		return m.RequestID, true
	case *wire.TrackStatus:
		return m.RequestID, true
	case *wire.PublishNamespace:
		return m.RequestID, true
	case *wire.SubscribeNamespace:
		return m.RequestID, true
	case *wire.SubscribeTracks:
		return m.RequestID, true
	}
	return 0, false
}

func (s *Session) localRequestIDParity() uint64 {
	return uint64(s.conn.Perspective()) % 2
}

func (s *Session) peerRequestIDParity() uint64 {
	return (uint64(s.conn.Perspective()) + 1) % 2
}

func (s *Session) validatePeerRequestID(id uint64) error {
	if id%2 != s.peerRequestIDParity() {
		return &SessionError{Code: uint64(ErrorCodeInvalidRequestID), Reason: "invalid request ID parity", Remote: false}
	}

	s.peerRequestIDsLock.Lock()
	defer s.peerRequestIDsLock.Unlock()
	if _, ok := s.peerRequestIDs[id]; ok {
		return &SessionError{Code: uint64(ErrorCodeInvalidRequestID), Reason: "duplicate request ID", Remote: false}
	}
	s.peerRequestIDs[id] = struct{}{}
	if id > s.largestPeerRequestID {
		s.largestPeerRequestID = id
	}
	return nil
}

func (s *Session) handleBidiStream(stream quic.Stream) {
	s.logger.Debug("accepted new bidi stream", "streamID", stream.StreamID())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-s.ctx.Done():
			stream.Stop(uint32(StreamResetErrorCodeSessionClosed))
			stream.Reset(uint32(StreamResetErrorCodeSessionClosed))
		}
	})

	parser, err := wire.NewParser(stream, uint64(s.version), wire.StreamTypeRequest)
	if err != nil {
		stream.Stop(uint32(StreamResetErrorCodeInternal))
		stream.Reset(uint32(StreamResetErrorCodeInternal))
		return
	}
	msg, err := parser.Read()
	if err != nil {
		stream.Stop(uint32(StreamResetErrorCodeInternal))
		stream.Reset(uint32(StreamResetErrorCodeInternal))
		return
	}
	if requestID, ok := requestIDOfMessage(msg); ok {
		if err := s.validatePeerRequestID(requestID); err != nil {
			s.closeWithError(err)
			stream.Stop(uint32(StreamResetErrorCodeInternal))
			stream.Reset(uint32(StreamResetErrorCodeInternal))
			return
		}
		if s.goAwaySent.Load() {
			s.rejectRequest(stream, requestID, RequestErrorCodeGoingAway, "going away")
			return
		}
	}

	switch m := msg.(type) {
	case *wire.TrackStatus:
		s.rejectRequest(stream, m.RequestID, RequestErrorCodeNotSupported, "not supported")
	case *wire.Subscribe:
		// TODO: Handle incoming request
		if s.handler == nil {
			return
		}
		request := newIncomingSubscribeRequest(m, s, &requestStream{parser, wire.NewAppender(stream, uint64(s.version)), stream, stream})
		s.handler.HandleSubscribe(request)
		request.readMessages()
	case *wire.Publish:
		s.rejectRequest(stream, m.RequestID, RequestErrorCodeNotSupported, "not supported")
	case *wire.Fetch:
		s.rejectRequest(stream, m.RequestID, RequestErrorCodeNotSupported, "not supported")
	case *wire.PublishNamespace:
		s.rejectRequest(stream, m.RequestID, RequestErrorCodeNotSupported, "not supported")
	case *wire.SubscribeNamespace:
		s.rejectRequest(stream, m.RequestID, RequestErrorCodeNotSupported, "not supported")
	case *wire.SubscribeTracks:
		s.rejectRequest(stream, m.RequestID, RequestErrorCodeNotSupported, "not supported")
	default:
		s.closeWithError(&SessionError{Code: uint64(ErrorCodeProtocolViolation), Reason: fmt.Sprintf("unexpected message type: %T", m), Remote: false})
		return
	}
}

// rejectRequest answers a request with REQUEST_ERROR and finishes the stream.
func (s *Session) rejectRequest(stream quic.Stream, requestID uint64, code RequestErrorCode, reason string) {
	s.logger.Debug("rejecting request", "streamID", stream.StreamID(), "requestID", requestID, "code", code, "reason", reason)
	appender := wire.NewAppender(stream, uint64(s.version))
	if err := appender.Write(&wire.RequestError{
		ErrorCode:   uint64(code),
		ErrorReason: reason,
	}); err != nil {
		stream.Reset(uint32(StreamResetErrorCodeInternal))
		return
	}
	_ = stream.Close()
}

func (s *Session) receiveDatagram(msg *wire.DatagramObject) {
	status := ObjectStatus(msg.ObjectStatus)
	if err := validateObjectStatus(status, msg.Properties); err != nil {
		s.closeWithError(err)
		return
	}
	priority := defaultPublisherPriority
	if !msg.DefaultPriority() {
		priority = msg.PublisherPriority
	}
	payload := make([]byte, len(msg.ObjectPayload))
	copy(payload, msg.ObjectPayload)
	s.pushDatagramObject(msg.TrackAlias, &Object{
		GroupID:              msg.GroupID,
		ObjectID:             msg.ObjectID,
		ForwardingPreference: ObjectForwardingPreferenceDatagram,
		PublisherPriority:    priority,
		Status:               status,
		EndOfGroup:           msg.EndOfGroup(),
		Payload:              bytes.NewReader(payload),
	})
}

func validateObjectStatus(status ObjectStatus, properties []wire.KeyValuePair) error {
	if !status.valid() {
		return &SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: fmt.Sprintf("unknown object status: %v", uint64(status)),
		}
	}
	if status != ObjectStatusNormal && len(properties) > 0 {
		return &SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: "properties on object with non-normal status",
		}
	}
	return nil
}

// getOrCreateEntry returns the entry of trackAlias, creating a pending one if
// needed. Exceeding the pending track limit is a session error.
func (s *Session) getOrCreateEntry(trackAlias uint64) (*trackEntry, error) {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()
	entry, ok := s.tracks[trackAlias]
	if ok {
		return entry, nil
	}
	if s.pendingTracks >= s.maxPendingTracks {
		return nil, &SessionError{
			Code:   uint64(ErrorCodeInternal),
			Reason: "too many unbound track aliases",
		}
	}
	entry = newTrackEntry()
	s.tracks[trackAlias] = entry
	s.pendingTracks++
	return entry, nil
}

// waitForReceiver blocks until trackAlias is bound to a receiver.
func (s *Session) waitForReceiver(trackAlias uint64) (objectReceiver, error) {
	entry, err := s.getOrCreateEntry(trackAlias)
	if err != nil {
		s.closeWithError(err)
		return nil, err
	}
	return entry.waitForReceiver(s.ctx)
}

func (s *Session) pushDatagramObject(trackAlias uint64, o *Object) {
	entry, err := s.getOrCreateEntry(trackAlias)
	if err != nil {
		s.closeWithError(err)
		return
	}
	if !entry.pushDatagram(o, s.maxPendingObjects) {
		s.logger.Info("pending object buffer overflow: dropping incoming object", "trackAlias", trackAlias)
	}
}

func (s *Session) bindTrackAlias(trackAlias uint64, r objectReceiver) error {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()

	entry, pending := s.tracks[trackAlias]
	if !pending {
		entry = newTrackEntry()
		s.tracks[trackAlias] = entry
	}
	if err := entry.bind(r); err != nil {
		return err
	}
	if pending {
		s.pendingTracks--
	}
	return nil
}

// removeReceiver drops the track alias entry of r, if it has one.
func (s *Session) removeReceiver(r objectReceiver) {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()

	for trackAlias, entry := range s.tracks {
		if entry.boundTo(r) {
			delete(s.tracks, trackAlias)
			return
		}
	}
}

func (s *Session) Subscribe(
	ctx context.Context,
	namespace [][]byte,
	name string,
	options ...OutgoingSubscribeRequestOption,
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
	rs, err := newRequestStream(stream, uint64(s.version))
	if err != nil {
		return nil, err
	}

	request, err := newOutgoingSubscribeRequest(requestID, s, rs, namespace, []byte(name), options...)
	if err != nil {
		return nil, err
	}

	if err := s.goTracked(request.readMessages); err != nil {
		return nil, err
	}

	select {
	case err := <-request.response:
		if err != nil {
			return nil, err
		}
		return request, nil
	case <-ctx.Done():
		_ = request.Close()
		return nil, context.Cause(ctx)
	case <-s.ctx.Done():
		s.closeLock.Lock()
		defer s.closeLock.Unlock()
		return nil, s.closeErr
	}
}

// validateGoAwayURI checks the new session URI of a received GOAWAY. A
// non-nil result is the error the session must be closed with.
func (s *Session) validateGoAwayURI(uri string) *SessionError {
	if len(uri) > maxGoAwayURILength {
		return &SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: "GOAWAY new session URI too long",
		}
	}
	if uri != "" && s.conn.Perspective() == quic.PerspectiveServer {
		return &SessionError{
			Code:   uint64(ErrorCodeProtocolViolation),
			Reason: "GOAWAY with new session URI received by server",
		}
	}
	return nil
}

// onGoAway validates a GOAWAY received on the control stream and hands it to
// the handler. A non-nil result is the error the session must be closed with.
func (s *Session) onGoAway(msg *wire.GoAwayCtrl) *SessionError {
	if err := s.validateGoAwayURI(msg.NewSessionURI); err != nil {
		return err
	}
	if msg.RequestID%2 != s.localRequestIDParity() {
		return &SessionError{
			Code:   uint64(ErrorCodeInvalidRequestID),
			Reason: "invalid GOAWAY request ID parity",
		}
	}
	if s.handler != nil {
		s.handler.HandleGoAway(msg.NewSessionURI, time.Duration(msg.Timeout)*time.Millisecond)
	}
	return nil
}

// GoAway sends GOAWAY on the control stream to tell the peer that the session
// is going to be closed. Only servers may pass a non-empty uri, it names the
// session the peer should migrate to. timeout is announced to the peer as the
// time the session will stay open, zero means no specific timeout. No timer
// runs, the caller is expected to close the session with
// ErrorCodeGoAwayTimeout itself once the timeout expired. Requests arriving
// after GoAway are rejected with REQUEST_ERROR GOING_AWAY.
func (s *Session) GoAway(uri string, timeout time.Duration) error {
	if uri != "" && s.conn.Perspective() == quic.PerspectiveClient {
		return ErrGoAwayURIFromClient
	}
	s.closeLock.Lock()
	if s.closeErr != nil {
		s.closeLock.Unlock()
		return s.closeErr
	}
	s.closeLock.Unlock()

	if !s.goAwaySent.CompareAndSwap(false, true) {
		return ErrGoAwaySent
	}
	s.peerRequestIDsLock.Lock()
	requestID := s.peerRequestIDParity()
	if len(s.peerRequestIDs) > 0 {
		requestID = s.largestPeerRequestID + 2
	}
	s.peerRequestIDsLock.Unlock()

	s.logger.Debug("sending GOAWAY", "uri", uri, "timeout", timeout, "requestID", requestID)
	err := s.localControlStream.write(s.ctx, &wire.GoAwayCtrl{
		NewSessionURI: uri,
		Timeout:       uint64(timeout.Milliseconds()),
		RequestID:     requestID,
	})
	if err != nil {
		s.closeOnError(err)
	}
	return err
}
