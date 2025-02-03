package moqtransport

import (
	"context"
	"errors"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errSetupFailed                 = errors.New("setup not done after first message exchange")
	errControlMessageQueueOverflow = errors.New("control message if full, message not queued")
)

type transportConfig struct {
	callbacks      *callbacks
	sessionOptions []sessionOption
}

type TransportOption func(*transportConfig)

func OnRequest(h Handler) TransportOption {
	return func(t *transportConfig) {
		t.callbacks.handler = h
	}
}

func Path(p string) TransportOption {
	return func(tc *transportConfig) {
		tc.sessionOptions = append(tc.sessionOptions, pathParameterOption(p))
	}
}

type Transport struct {
	ctx       context.Context
	cancelCtx context.CancelCauseFunc

	logger *slog.Logger

	conn          Connection
	controlStream Stream

	ctrlMsgSendQueue chan wire.ControlMessage

	session *session

	callbacks *callbacks
}

func NewWebTransportClientTransport(conn Connection, opts ...TransportOption) (*Transport, error) {
	return NewTransport(conn, false, false, opts...)
}

func NewQUICClientTransport(conn Connection, opts ...TransportOption) (*Transport, error) {
	return NewTransport(conn, false, true, opts...)
}

func NewWebTransportServerTransport(conn Connection, opts ...TransportOption) (*Transport, error) {
	return NewTransport(conn, true, false, opts...)
}

func NewQUICServerTransport(conn Connection, opts ...TransportOption) (*Transport, error) {
	return NewTransport(conn, true, true, opts...)
}

func NewTransport(
	conn Connection,
	isServer, isQUIC bool,
	opts ...TransportOption,
) (*Transport, error) {
	cb := &callbacks{}
	tc := &transportConfig{
		callbacks: cb,
	}
	for _, opt := range opts {
		opt(tc)
	}
	session, err := newSession(cb, isServer, isQUIC)
	if err != nil {
		return nil, err
	}
	return newTransportWithSession(conn, session, tc)
}

func newTransportWithSession(
	conn Connection,
	session *session,
	tc *transportConfig,
) (*Transport, error) {
	var ctrlStream Stream
	var err error
	if session.isServer {
		ctrlStream, err = conn.AcceptStream(context.Background())
		if err != nil {
			return nil, err
		}
	}
	dir := "server"
	if !session.isServer {
		dir = "client"
		ctrlStream, err = conn.OpenStreamSync(context.Background())
		if err != nil {
			return nil, err
		}
	}
	ctx, cancelCtx := context.WithCancelCause(context.Background())
	t := &Transport{
		ctx:              ctx,
		cancelCtx:        cancelCtx,
		logger:           defaultLogger.With("dir", dir),
		conn:             conn,
		controlStream:    ctrlStream,
		ctrlMsgSendQueue: make(chan wire.ControlMessage, 100),
		session:          session,
		callbacks:        tc.callbacks,
	}
	tc.callbacks.t = t

	if !session.isServer {
		if err = session.sendClientSetup(); err != nil {
			return nil, err
		}
	}
	go t.sendCtrlMsgs()

	parser := wire.NewControlMessageParser(t.controlStream)
	msg, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	if err = t.recvCtrlMsg(msg); err != nil {
		return nil, err
	}
	if !t.session.setupDone {
		return nil, errSetupFailed
	}

	go t.readControlStream(parser)
	go t.readStreams()
	go t.readDatagrams()

	return t, nil
}

func (t *Transport) destroy(err error) {
	t.cancelCtx(err)
}

func (t *Transport) recvCtrlMsg(msg wire.ControlMessage) error {
	t.logControlMessage(msg, false)
	return t.session.onControlMessage(msg)
}

func (t *Transport) queueCtrlMessage(msg wire.ControlMessage) error {
	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	case t.ctrlMsgSendQueue <- msg:
		return nil
	default:
		return errControlMessageQueueOverflow
	}
}

func (t *Transport) sendCtrlMsgs() {
	for msg := range t.ctrlMsgSendQueue {
		t.logControlMessage(msg, true)
		_, err := t.controlStream.Write(compileMessage(msg))
		if err != nil {
			t.destroy(err)
		}
	}
}

func (t *Transport) readControlStream(parser *wire.ControlMessageParser) {
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
		go t.handleUniStream(stream)
	}
}

func (t *Transport) handleUniStream(stream ReceiveStream) {
	parser, err := wire.NewObjectStreamParser(stream)
	if err != nil {
		stream.Stop(0) // TODO: Set correct error and possibly destroy session?
		return
	}
	switch parser.Typ {
	case wire.StreamTypeFetch:
	case wire.StreamTypeSubgroup:
		if err := t.readSubgroupStream(parser); err != nil {
			stream.Stop(0) // TODO: Set correct error and possibly destroy session?
			return
		}
	default:
		stream.Stop(0) // TODO: Set correct error and possibly destroy session?
		return
	}
}

func (t *Transport) readSubgroupStream(parser *wire.ObjectStreamParser) error {
	sid, err := parser.SubscribeID()
	if err != nil {
		return err
	}

	subscription, ok := t.session.remoteTrackBySubscribeID(sid)
	if !ok {
		return errUnknownSubscribeID
	}

	for m, err := range parser.Messages() {
		if err != nil {
			return err
		}
		subscription.push(&Object{
			GroupID:    m.GroupID,
			SubGroupID: m.SubgroupID,
			ObjectID:   m.ObjectID,
		})
	}
	return nil
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

func (t *Transport) Path() string {
	return t.session.path
}

func (t *Transport) SubscribeAnnouncements(ctx context.Context, prefix []string) error {
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

func (t *Transport) Subscribe(
	ctx context.Context,
	id, alias uint64,
	namespace []string,
	name string,
	auth string,
) (*RemoteTrack, error) {
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

func (t *Transport) Fetch(
	ctx context.Context,
	id uint64,
	namespace []string,
	name string,
) (*RemoteTrack, error) {
	f := &subscription{
		ID:        id,
		Namespace: namespace,
		Trackname: name,
		isFetch:   true,
		response:  make(chan subscriptionResponse),
	}
	return t.subscribe(ctx, f)
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

func (t *Transport) Announce(ctx context.Context, namespace []string) error {
	a := &announcement{
		Namespace:  namespace,
		parameters: map[uint64]wire.Parameter{},
		response:   make(chan announcementResponse, 1),
	}
	if err := t.session.announce(a); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-a.response:
		return res.err
	}
}

func (t *Transport) acceptAnnouncement(ns []string) error {
	return t.session.acceptAnnouncement(ns)
}

func (t *Transport) rejectAnnouncement(ns []string, c uint64, r string) error {
	return t.session.rejectAnnouncement(ns, c, r)
}

func (t *Transport) acceptAnnouncementSubscription(as announcementSubscription) error {
	return t.session.acceptAnnouncementSubscription(as)
}

func (t *Transport) rejectAnnouncementSubscription(as announcementSubscription, c uint64, r string) error {
	return t.session.rejectAnnouncementSubscription(as, c, r)
}

func (t *Transport) acceptSubscription(id uint64, lt *LocalTrack) error {
	return t.session.acceptSubscription(id, lt)
}

func (t *Transport) rejectSubscription(id uint64, code uint64, reason string) error {
	return t.session.rejectSubscription(id, code, reason)
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
