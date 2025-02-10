package moqtransport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

var (
	// ErrControlMessageQueueOverflow is returned if a control message cannot be
	// send due to queue overflow.
	ErrControlMessageQueueOverflow = errors.New("control message overflow, message not queued")
)

type sessionI interface {
	getPath() string
	getSetupDone() bool
	onControlMessage(wire.ControlMessage) error
	remoteTrackBySubscribeID(uint64) (*RemoteTrack, bool)
	remoteTrackByTrackAlias(uint64) (*RemoteTrack, bool)
	announce(*announcement) error
	subscribeAnnounces(*announcementSubscription) error
	unsubscribeAnnounces([]string) error
	subscribe(*subscription) error
	acceptAnnouncement([]string) error
	rejectAnnouncement([]string, uint64, string) error
	cancelAnnouncement(namespace []string, errorCode uint64, reason string) error
	acceptAnnouncementSubscription([]string) error
	rejectAnnouncementSubscription([]string, uint64, string) error
	acceptSubscription(uint64, *localTrack) error
	rejectSubscription(uint64, uint64, string) error
	subscriptionDone(id, code, count uint64, reason string) error
	unannounce([]string) error
	sendClientSetup() error
}

type sessionFactory func(sessionCallbacks, Perspective, Protocol, ...sessionOption) (sessionI, error)

type controlMessageParserI interface {
	Parse() (wire.ControlMessage, error)
}

type controlMessageParserFactory func(io.Reader) controlMessageParserI

type transportConfig struct {
	callbacks               *callbacks
	sessionOptions          []sessionOption
	newSession              sessionFactory
	newControlMessageParser controlMessageParserFactory
	datagramsDisabled       bool
}

// A TransportOption sets a configuration parameter of a Transport.
type TransportOption func(*transportConfig)

// OnRequest sets the handler which should be called on incoming MoQ messages.
func OnRequest(h Handler) TransportOption {
	return func(t *transportConfig) {
		t.callbacks.handler = h
	}
}

// Path sets the path parameter of a MoQ transport session when using QUIC. To
// set the path of a session using WebTransport, it has to be set in the HTTP
// server/client when opening the connection.
func Path(p string) TransportOption {
	return func(tc *transportConfig) {
		tc.sessionOptions = append(tc.sessionOptions, pathParameterOption(p))
	}
}

func MaxSubscribeID(id uint64) TransportOption {
	return func(tc *transportConfig) {
		tc.sessionOptions = append(tc.sessionOptions, maxSubscribeIDOption(id))
	}
}

func DisableDatagrams() TransportOption {
	return func(tc *transportConfig) {
		tc.datagramsDisabled = true
	}
}

func setSessionFactory(f sessionFactory) TransportOption {
	return func(tc *transportConfig) {
		tc.newSession = f
	}
}

func setControlMessageParserFactory(f controlMessageParserFactory) TransportOption {
	return func(tc *transportConfig) {
		tc.newControlMessageParser = f
	}
}

// A Transport is an endpoint of a MoQ Transport session.
type Transport struct {
	ctx           context.Context
	cancelCtx     context.CancelCauseFunc
	wg            sync.WaitGroup
	initLock      sync.Mutex
	handshakeDone chan struct{}
	destroyOnce   sync.Once

	logger *slog.Logger

	conn          Connection
	controlStream Stream

	ctrlMsgSendQueue chan wire.ControlMessage

	session sessionI

	callbacks *callbacks
}

func NewTransport(
	conn Connection,
	opts ...TransportOption,
) (*Transport, error) {
	cb := &callbacks{}
	tc := &transportConfig{
		callbacks:      cb,
		sessionOptions: []sessionOption{},
		newSession: func(sc sessionCallbacks, p1 Perspective, p2 Protocol, so ...sessionOption) (sessionI, error) {
			return newSession(sc, p1, p2, so...)
		},
		newControlMessageParser: func(r io.Reader) controlMessageParserI {
			return wire.NewControlMessageParser(r)
		},
	}
	for _, opt := range opts {
		opt(tc)
	}
	session, err := tc.newSession(cb, conn.Perspective(), conn.Protocol(), tc.sessionOptions...)
	if err != nil {
		return nil, err
	}
	return newTransportWithSession(conn, session, tc)
}

func newTransportWithSession(
	conn Connection,
	session sessionI,
	tc *transportConfig,
) (*Transport, error) {
	ctx, cancelCtx := context.WithCancelCause(context.Background())
	t := &Transport{
		ctx:              ctx,
		cancelCtx:        cancelCtx,
		wg:               sync.WaitGroup{},
		initLock:         sync.Mutex{},
		handshakeDone:    make(chan struct{}),
		logger:           defaultLogger.With("perspective", conn.Perspective()),
		conn:             conn,
		controlStream:    nil,
		ctrlMsgSendQueue: make(chan wire.ControlMessage, 100),
		session:          session,
		callbacks:        tc.callbacks,
	}
	tc.callbacks.t = t
	go t.init(tc)
	return t, nil
}

func (t *Transport) init(tc *transportConfig) {
	var err error
	if t.conn.Perspective() == PerspectiveServer {
		t.controlStream, err = t.conn.AcceptStream(t.ctx)
		if err != nil {
			t.destroy(err)
			return
		}
	}
	if t.conn.Perspective() == PerspectiveClient {
		t.controlStream, err = t.conn.OpenStreamSync(t.ctx)
		if err != nil {
			t.destroy(err)
			return
		}
		if err = t.session.sendClientSetup(); err != nil {
			t.destroy(err)
			return
		}
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.sendCtrlMsgs()
	}()

	parser := tc.newControlMessageParser(t.controlStream)
	msg, err := parser.Parse()
	if err != nil {
		t.destroy(err)
		return
	}
	if err = t.recvCtrlMsg(msg); err != nil {
		t.destroy(err)
		return
	}
	if !t.session.getSetupDone() {
		t.destroy(err)
		t.logger.Info("setup not done after handshake, closing")
		return
	}
	close(t.handshakeDone)

	t.wg.Add(3)
	go func() {
		defer t.wg.Done()
		t.readControlStream(parser)
	}()
	go func() {
		defer t.wg.Done()
		t.readStreams()
	}()
	go func() {
		defer t.wg.Done()
		if !tc.datagramsDisabled {
			t.readDatagrams()
		}
	}()
}

func (t *Transport) Close() error {
	defer t.wg.Wait()
	t.destroy(nil)
	return nil
}

// TODO: Propagate error to application so it can close the connection
func (t *Transport) destroy(err error) {
	t.destroyOnce.Do(func() {
		t.logger.Info("destroying transport", "error", err)
		t.cancelCtx(err)
		var pe ProtocolError
		var code uint64
		var reason string
		if errors.As(err, &pe) {
			code = pe.code
			reason = pe.message
		}
		err = t.conn.CloseWithError(code, reason)
		if err != nil {
			t.logger.Info("error on connection close", "error", err)
		}
	})
}

func (t *Transport) recvCtrlMsg(msg wire.ControlMessage) error {
	t.logger.Info("received message", "type", msg.Type().String(), "msg", msg)
	return t.session.onControlMessage(msg)
}

func (t *Transport) queueCtrlMessage(msg wire.ControlMessage) error {
	select {
	case <-t.ctx.Done():
		return context.Cause(t.ctx)
	case t.ctrlMsgSendQueue <- msg:
		return nil
	default:
		return ErrControlMessageQueueOverflow
	}
}

func (t *Transport) sendCtrlMsgs() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-t.ctrlMsgSendQueue:
			t.logger.Info("sending message", "type", msg.Type().String(), "msg", msg)
			_, err := t.controlStream.Write(compileMessage(msg))
			if err != nil {
				t.destroy(err)
				return
			}
		}
	}
}

func (t *Transport) readControlStream(parser controlMessageParserI) {
	for {
		msg, err := parser.Parse()
		if err != nil {
			t.destroy(err)
			return
		}
		if err = t.recvCtrlMsg(msg); err != nil {
			t.destroy(err)
			return
		}
	}
}

func (t *Transport) readStreams() {
	for {
		stream, err := t.conn.AcceptUniStream(context.Background())
		if err != nil {
			t.destroy(err)
			return
		}
		t.logger.Info("got new uni stream")
		go t.handleUniStream(stream)
	}
}

func (t *Transport) handleUniStream(stream ReceiveStream) {
	t.logger.Info("handling new uni stream")
	parser, err := wire.NewObjectStreamParser(stream)
	if err != nil {
		t.destroy(ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: err.Error(),
		})
		return
	}
	t.logger.Info("parsed object stream header")
	switch parser.Typ {
	case wire.StreamTypeFetch:
		err = t.readFetchStream(parser)
	case wire.StreamTypeSubgroup:
		err = t.readSubgroupStream(parser)
	}
	if err != nil {
		t.destroy(ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "failed to parse object",
		})
	}
}

func (t *Transport) readFetchStream(parser *wire.ObjectStreamParser) error {
	t.logger.Info("reading fetch stream")
	sid, err := parser.SubscribeID()
	if err != nil {
		t.logger.Info("failed to parse subscribe ID", "error", err)
		return err
	}
	rt, ok := t.session.remoteTrackBySubscribeID(sid)
	if !ok {
		return errUnknownSubscribeID
	}
	return rt.readFetchStream(parser)
}

func (t *Transport) readSubgroupStream(parser *wire.ObjectStreamParser) error {
	t.logger.Info("reading subgroup")
	sid, err := parser.TrackAlias()
	if err != nil {
		t.logger.Info("failed to parse subscribe ID", "error", err)
		return err
	}
	rt, ok := t.session.remoteTrackByTrackAlias(sid)
	if !ok {
		return errUnknownSubscribeID
	}
	return rt.readSubgroupStream(parser)
}

func (t *Transport) readDatagrams() {
	msg := new(wire.ObjectMessage)
	for {
		dgram, err := t.conn.ReceiveDatagram(context.Background())
		if err != nil {
			t.destroy(err)
		}
		_, err = msg.ParseDatagram(dgram)
		if err != nil {
			t.logger.Error("failed to parse datagram object", "error", err)
			t.destroy(errUnknownSubscribeID)
			return
		}
		subscription, ok := t.session.remoteTrackByTrackAlias(msg.TrackAlias)
		if !ok {
			t.destroy(errUnknownSubscribeID)
			return
		}
		subscription.push(&Object{
			GroupID:    msg.GroupID,
			SubGroupID: msg.SubgroupID,
			ObjectID:   msg.ObjectID,
			Payload:    msg.ObjectPayload,
		})
	}
}

// Local API

func (t *Transport) waitForHandshakeDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.handshakeDone:
		return nil
	}
}

// Announce announces namespace to the peer. It blocks until a response from the
// peer was received or ctx is cancelled and returns an error if the
// announcement was rejected.
func (t *Transport) Announce(ctx context.Context, namespace []string) error {
	if err := t.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	a := &announcement{
		Namespace:  namespace,
		parameters: map[uint64]wire.Parameter{},
		response:   make(chan error, 1),
	}
	if err := t.session.announce(a); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-a.response:
		return res
	}
}

func (t *Transport) AnnounceCancel(namespace []string, errorCode uint64, reason string) error {
	return t.session.cancelAnnouncement(namespace, errorCode, reason)
}

// Fetch fetches track in namespace from the peer using id as the subscribe ID.
// It blocks until a response from the peer was received or ctx is cancelled.
func (t *Transport) Fetch(
	ctx context.Context,
	id uint64,
	namespace []string,
	track string,
) (*RemoteTrack, error) {
	if err := t.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	f := &subscription{
		ID:        id,
		Namespace: namespace,
		Trackname: track,
		isFetch:   true,
		response:  make(chan subscriptionResponse, 1),
	}
	return t.subscribe(ctx, f)
}

// Path returns the path of the MoQ session which was exchanged during the
// handshake when using QUIC.
func (t *Transport) Path() string {
	if err := t.waitForHandshakeDone(t.ctx); err != nil {
		return ""
	}
	return t.session.getPath()
}

func (t *Transport) RequestTrackStatus() {
	// TODO
}

// Subscribe subscribes to track in namespace using id as the subscribe ID. It
// blocks until a response from the peer was received or ctx is cancelled.
func (t *Transport) Subscribe(
	ctx context.Context,
	id, alias uint64,
	namespace []string,
	name string,
	auth string,
) (*RemoteTrack, error) {
	if err := t.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	ps := &subscription{
		ID:            id,
		TrackAlias:    alias,
		Namespace:     namespace,
		Trackname:     name,
		Authorization: auth,
		Expires:       0,
		GroupOrder:    0,
		ContentExists: false,
		response:      make(chan subscriptionResponse, 1),
	}
	return t.subscribe(ctx, ps)
}

// SubscribeAnnouncements subscribes to announcements of namespaces with prefix.
// It blocks until a response from the peer is received or ctx is cancelled.
func (t *Transport) SubscribeAnnouncements(ctx context.Context, prefix []string) error {
	if err := t.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	as := &announcementSubscription{
		namespace: prefix,
		response:  make(chan announcementSubscriptionResponse, 1),
	}
	if err := t.session.subscribeAnnounces(as); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-as.response:
		return resp.err
	}
}

func (t *Transport) Unannounce(ctx context.Context, namespace []string) error {
	if err := t.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	return t.session.unannounce(namespace)
}

func (t *Transport) UnsubscribeAnnouncements(ctx context.Context, namespace []string) error {
	return t.session.unsubscribeAnnounces(namespace)
}

func (t *Transport) subscribe(
	ctx context.Context,
	ps *subscription,
) (*RemoteTrack, error) {
	if err := t.session.subscribe(ps); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ps.response:
		return res.track, res.err
	}
}

func (t *Transport) acceptAnnouncement(ns []string) error {
	return t.session.acceptAnnouncement(ns)
}

func (t *Transport) rejectAnnouncement(ns []string, c uint64, r string) error {
	return t.session.rejectAnnouncement(ns, c, r)
}

func (t *Transport) acceptAnnouncementSubscription(as []string) error {
	return t.session.acceptAnnouncementSubscription(as)
}

func (t *Transport) rejectAnnouncementSubscription(as []string, c uint64, r string) error {
	return t.session.rejectAnnouncementSubscription(as, c, r)
}

func (t *Transport) acceptSubscription(id uint64, lt *localTrack) error {
	return t.session.acceptSubscription(id, lt)
}

func (t *Transport) rejectSubscription(id uint64, code uint64, reason string) error {
	return t.session.rejectSubscription(id, code, reason)
}

func (t *Transport) subscriptionDone(id, code, streamCount uint64, reason string) error {
	return t.session.subscriptionDone(id, code, streamCount, reason)
}

func compileMessage(msg wire.ControlMessage) []byte {
	buf := make([]byte, 16, 1500)
	buf = append(buf, msg.Append(buf[16:])...)
	length := len(buf[16:])

	typeLenBuf := quicvarint.Append(buf[:0], uint64(msg.Type()))
	typeLenBuf = quicvarint.Append(typeLenBuf, uint64(length))

	n := copy(buf[0:16], typeLenBuf)
	buf = append(buf[:n], buf[16:]...)

	return buf
}
