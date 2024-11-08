package moqtransport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

type pendingSubscription struct {
	response chan error
}

type TransportOption func(*Transport)

func OnSubscription(h SubscriptionHandler) TransportOption {
	return func(t *Transport) {
		t.subscriptionHandler = h
	}
}

func OnAnnouncement(h AnnouncementHandler) TransportOption {
	return func(t *Transport) {
		t.announcementHandler = h
	}
}

type Transport struct {
	logger *slog.Logger

	conn          Connection
	controlStream Stream

	setupDone chan struct{}

	ctrlMsgSendQueue chan wire.ControlMessage
	ctrlMsgRecvQueue chan wire.ControlMessage

	session *Session

	subscriptionHandler SubscriptionHandler
	announcementHandler AnnouncementHandler
}

func NewTransport(
	ctx context.Context,
	conn Connection,
	isServer, isQUIC bool,
	opts ...TransportOption,
) (*Transport, error) {
	session, err := NewSession(isServer, isQUIC)
	if err != nil {
		return nil, err
	}
	return NewTransportWithSession(ctx, conn, session, opts...)
}

func NewTransportWithSession(
	ctx context.Context,
	conn Connection,
	session *Session,
	opts ...TransportOption,
) (*Transport, error) {
	var ctrlStream Stream
	var err error
	if session.isServer {
		ctrlStream, err = conn.AcceptStream(ctx)
		if err != nil {
			return nil, err
		}
	}
	dir := "server"
	if !session.isServer {
		dir = "client"
		ctrlStream, err = conn.OpenStreamSync(ctx)
		if err != nil {
			return nil, err
		}
	}
	t := &Transport{
		logger:              defaultLogger.With("dir", dir),
		conn:                conn,
		controlStream:       ctrlStream,
		setupDone:           make(chan struct{}),
		ctrlMsgSendQueue:    make(chan wire.ControlMessage),
		ctrlMsgRecvQueue:    make(chan wire.ControlMessage),
		session:             session,
		subscriptionHandler: nil,
		announcementHandler: nil,
	}
	for _, opt := range opts {
		opt(t)
	}
	go func() {
		if err := t.sendCtrlMsgs(); err != nil {
			panic(err)
		}
	}()
	go t.readControlStream()
	go t.readSubscriptions()
	go t.readStreams(ctx)
	go t.readDatagrams(ctx)

	select {
	case <-t.setupDone:
		return t, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *Transport) Close() error {
	panic("TODO")
}

func (t *Transport) recvCtrlMsg(msg wire.ControlMessage) error {
	t.logControlMessage(msg, false)
	return t.session.OnControlMessage(msg)
}

func (t *Transport) sendCtrlMsgs() error {
	for msg := range t.session.SendControlMessages() {
		t.logControlMessage(msg, true)
		_, err := t.controlStream.Write(compileMessage(msg))
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Transport) readControlStream() error {
	parser := wire.NewControlMessageParser(t.controlStream)
	msg, err := parser.Parse()
	if err != nil {
		return err
	}
	if err = t.recvCtrlMsg(msg); err != nil {
		return err
	}
	if !t.session.setupDone {
		return errors.New("setup not done after first message exchange")
	}
	close(t.setupDone)
	for {
		msg, err := parser.Parse()
		if err != nil {
			return err
		}
		if err = t.recvCtrlMsg(msg); err != nil {
			return err
		}
	}
}

func (t *Transport) readStreams(ctx context.Context) error {
	for {
		stream, err := t.conn.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		go t.handleUniStream(ctx, stream)
	}
}

func (t *Transport) handleUniStream(ctx context.Context, stream ReceiveStream) {
	parser, err := wire.NewObjectStreamParser(stream)
	if err != nil {
		panic(err)
	}
	switch parser.Typ {
	case wire.StreamTypeFetch:
	case wire.StreamTypeSubgroup:
		t.readSubgroupStream(parser)
	default:
		panic(fmt.Sprintf("unexpected wire.StreamType: %#v", parser.Typ))
	}
}

func (t *Transport) readSubgroupStream(parser *wire.ObjectStreamParser) error {
	sid, err := parser.SubscribeID()
	if err != nil {
		return err
	}

	subscription, ok := t.session.RemoteTrackBySubscribeID(sid)
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

func (t *Transport) readDatagrams(ctx context.Context) error {
	msg := new(wire.ObjectMessage)
	for {
		dgram, err := t.conn.ReceiveDatagram(ctx)
		if err != nil {
			return err
		}
		_, err = msg.ParseDatagram(dgram)
		if err != nil {
			t.logger.Error("failed to parse datagram object", "error", err)
		}
		subscription, ok := t.session.RemoteTrackByTrackAlias(msg.TrackAlias)
		if !ok {
			return errUnknownSubscribeID
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
	if err := t.session.Subscribe(ctx, ps); err != nil {
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

func (t *Transport) Announce(ctx context.Context, namespace []string) error {
	return nil
}

func (t *Transport) acceptAnnouncement(Announcement) {

}

func (t *Transport) rejectAnnouncement(Announcement, uint64, string) {

}

func (t *Transport) acceptSubscription(sub Subscription) error {
	return t.session.AcceptSubscription(sub)
}

func (t *Transport) rejectSubscription(sub Subscription, code uint64, reason string) error {
	return t.session.RejectSubscription(sub, code, reason)
}

func (t *Transport) unsubscribe(uint64) {

}

// Remote API

func (t *Transport) readSubscriptions() {
	for s := range t.session.IncomingSubscriptions() {
		if t.subscriptionHandler != nil {
			t.subscriptionHandler.HandleSubscription(t, s, &defaultSubscriptionResponseWriter{
				subscription: s,
				transport:    t,
			})
		} else {
			t.session.RejectSubscription(s, SubscribeErrorTrackDoesNotExist, "track not found")
		}
	}
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
