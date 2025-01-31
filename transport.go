package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errSetupNotDone                = errors.New("setup not done after first message exchange")
	errControlMessageQueueOverflow = errors.New("control message if full, message not queued")
)

type TransportOption func(*Transport)

func OnSubscription(h SubscriptionHandler) TransportOption {
	return func(t *Transport) {
		t.callbacks.subscriptionHandler = h
	}
}

func OnAnnouncement(h AnnouncementHandler) TransportOption {
	return func(t *Transport) {
		t.callbacks.announcementHandler = h
	}
}

func OnAnnouncementSubscription(h AnnouncementSubscriptionHandler) TransportOption {
	return func(t *Transport) {
		t.callbacks.announcementSubscriptionHandler = h
	}
}

type Transport struct {
	ctx       context.Context
	cancelCtx context.CancelCauseFunc

	logger *slog.Logger

	conn          Connection
	controlStream Stream

	setupDone chan struct{}

	ctrlMsgSendQueue chan wire.ControlMessage

	session   *session
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
	session, err := newSession(cb, isServer, isQUIC)
	if err != nil {
		return nil, err
	}
	return newTransportWithSession(conn, session, cb, opts...)
}

func newTransportWithSession(
	conn Connection,
	session *session,
	cb *callbacks,
	opts ...TransportOption,
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
		setupDone:        make(chan struct{}),
		ctrlMsgSendQueue: make(chan wire.ControlMessage, 100),
		session:          session,
		callbacks:        cb,
	}
	cb.t = t
	for _, opt := range opts {
		opt(t)
	}
	go t.sendCtrlMsgs()
	go t.readControlStream()
	go t.readStreams()
	go t.readDatagrams()

	select {
	case <-t.setupDone:
		return t, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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

func (t *Transport) readControlStream() {
	parser := wire.NewControlMessageParser(t.controlStream)
	msg, err := parser.Parse()
	if err != nil {
		t.destroy(err)
	}
	if err = t.recvCtrlMsg(msg); err != nil {
		t.destroy(err)
	}
	if !t.session.setupDone {
		t.destroy(errSetupNotDone)
	}
	close(t.setupDone)
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
		panic(err)
	}
	switch parser.Typ {
	case wire.StreamTypeFetch:
	case wire.StreamTypeSubgroup:
		if err := t.readSubgroupStream(parser); err != nil {
			panic("TODO: Close stream")
		}
	default:
		panic(fmt.Sprintf("unexpected wire.StreamType: %#v", parser.Typ))
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
			panic(err)
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

func (t *Transport) SubscribeAnnouncements(ctx context.Context, prefix []string) error {
	as := AnnouncementSubscription{
		namespace: prefix,
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
	ps := Subscription{
		ID:            id,
		TrackAlias:    alias,
		Namespace:     namespace,
		Trackname:     name,
		Authorization: auth,
		Expires:       0,
		GroupOrder:    0,
		ContentExists: false,
		publisher:     nil,
		response:      make(chan subscriptionResponse, 1),
	}
	if err := t.session.subscribe(ps); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ps.response:
		if res.err != nil {
			return nil, res.err
		}
		return res.track, nil
	}
}

func (t *Transport) Announce(namespace []string) error {
	return t.session.announce(namespace)
}

func (t *Transport) acceptAnnouncement(a Announcement) error {
	return t.session.acceptAnnouncement(a)
}

func (t *Transport) rejectAnnouncement(a Announcement, c uint64, r string) error {
	return t.session.rejectAnnouncement(a, c, r)
}

func (t *Transport) acceptAnnouncementSubscription(as AnnouncementSubscription) error {
	return t.session.acceptAnnouncementSubscription(as)
}

func (t *Transport) rejectAnnouncementSubscription(as AnnouncementSubscription, c uint64, r string) error {
	return t.session.rejectAnnouncementSubscription(as, c, r)

}

func (t *Transport) acceptSubscription(sub Subscription) error {
	return t.session.acceptSubscription(sub)
}

func (t *Transport) rejectSubscription(sub Subscription, code uint64, reason string) error {
	return t.session.rejectSubscription(sub, code, reason)
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
