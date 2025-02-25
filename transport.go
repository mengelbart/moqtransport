package moqtransport

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type Transport struct {
	logger *slog.Logger

	session *Session

	Conn Connection

	InitialMaxSubscribeID uint64
	DatagramsDisabled     bool

	Handler Handler

	ctx       context.Context
	cancelCtx context.CancelCauseFunc

	closeOnce sync.Once
}

func (t *Transport) NewSession(ctx context.Context) (*Session, error) {
	t.logger = defaultLogger.With("perspective", t.Conn.Perspective())
	t.logger.Info("NewSession")

	controlStream := newControlStream(t)

	t.ctx, t.cancelCtx = context.WithCancelCause(context.Background())
	t.session = t.newSession(controlStream)

	switch t.Conn.Perspective() {
	case PerspectiveServer:
		go controlStream.accept(t.Conn, t.session)
	case PerspectiveClient:
		if err := t.session.sendClientSetup(); err != nil {
			return nil, err
		}
		go controlStream.open(t.Conn, t.session)
	default:
		return nil, errors.New("invalid perspective")
	}

	t.logger.Info("control stream started")
	go t.readStreams()
	if !t.DatagramsDisabled {
		go t.readDatagrams()
	}
	return t.session, nil
}

func (t *Transport) handle(m *Message) {
	if t.Handler != nil {
		switch m.Method {
		case MessageSubscribe:
			srw := &subscriptionResponseWriter{
				id:         m.SubscribeID,
				trackAlias: m.TrackAlias,
				session:    t.session,
				localTrack: newLocalTrack(t.Conn, m.SubscribeID, m.TrackAlias, func(code, count uint64, reason string) error {
					return t.session.subscriptionDone(m.SubscribeID, code, count, reason)
				}),
				handled: false,
			}
			t.Handler.Handle(srw, m)
			if !srw.handled {
				if err := srw.Reject(0, "unhandled subscription"); err != nil {
					t.logger.Error("failed to reject subscription", "error", err)
				}
			}
		case MessageFetch:
			frw := &fetchResponseWriter{
				id:      m.SubscribeID,
				session: t.session,
				localTrack: newLocalTrack(t.Conn, m.SubscribeID, 0, func(code, count uint64, reason string) error {
					return t.session.subscriptionDone(m.SubscribeID, code, count, reason)
				}),
				handled: false,
			}
			t.Handler.Handle(frw, m)
			if !frw.handled {
				if err := frw.Reject(0, "unhandled fetch"); err != nil {
					t.logger.Error("failed to reject fetch", "error", err)
				}
			}
		case MessageAnnounce:
			arw := &announcementResponseWriter{
				namespace: m.Namespace,
				session:   t.session,
				handled:   false,
			}
			t.Handler.Handle(arw, m)
			if !arw.handled {
				if err := arw.Reject(0, "unhandled announce"); err != nil {
					t.logger.Error("failed to reject announce", "error", err)
				}
			}
		case MessageSubscribeAnnounces:
			asrw := &announcementSubscriptionResponseWriter{
				prefix:  m.Namespace,
				session: t.session,
				handled: false,
			}
			t.Handler.Handle(asrw, m)
			if !asrw.handled {
				if err := asrw.Reject(0, "unhandled announcement subscription"); err != nil {
					t.logger.Error("failed to reject announcement subscription", "error", err)
				}
			}
		default:
			t.Handler.Handle(nil, m)
		}
	}
}

func (t *Transport) newSession(cs *controlStream) *Session {
	return &Session{
		logger:                                   defaultLogger.With("perspective", t.Conn.Perspective()),
		ctx:                                      t.ctx,
		cancelCtx:                                t.cancelCtx,
		handshakeDoneCh:                          make(chan struct{}),
		controlMessageSender:                     cs,
		handler:                                  t,
		version:                                  0,
		protocol:                                 t.Conn.Protocol(),
		perspective:                              t.Conn.Perspective(),
		path:                                     "",
		outgoingAnnouncements:                    newAnnouncementMap(),
		incomingAnnouncements:                    newAnnouncementMap(),
		pendingOutgointAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		pendingIncomingAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		highestSubscribesBlocked:                 atomic.Uint64{},
		outgoingSubscriptions:                    newSubscriptionMap(0),
		incomingSubscriptions:                    newSubscriptionMap(t.InitialMaxSubscribeID),
	}
}

func (t *Transport) readStreams() {
	for {
		stream, err := t.Conn.AcceptUniStream(context.Background())
		if err != nil {
			t.logger.Error("failed to accept uni stream", "error", err)
			t.handleProtocolViolation(err)
			return
		}
		go func() {
			t.logger.Info("handling new uni stream")
			parser, err := wire.NewObjectStreamParser(stream)
			if err != nil {
				t.logger.Info("failed to read uni stream header", "error", err)
				return
			}
			t.logger.Info("parsed object stream header")
			if err := t.session.handleUniStream(parser); err != nil {
				t.logger.Error("session failed to handle uni stream", "error", err)
				return
			}
		}()
	}
}

func (t *Transport) readDatagrams() {
	for {
		dgram, err := t.Conn.ReceiveDatagram(context.Background())
		if err != nil {
			t.logger.Error("dgram receive error", "error", err)
			t.handleProtocolViolation(err)
			return
		}
		go func() {
			if err := t.session.receiveDatagram(dgram); err != nil {
				t.logger.Error("session failed to handle dgram", "error", err)
				t.handleProtocolViolation(err)
				return
			}
		}()
	}
}

func (t *Transport) handleProtocolViolation(err error) {
	t.cancelCtx(err)
	t.closeOnce.Do(func() {
		var pv ProtocolError
		code := ErrorCodeInternal
		message := "internal error"
		if errors.As(err, &pv) {
			code = pv.code
			message = pv.message
		}
		t.logger.Error("closing connection with error", "error", err)
		if err := t.Conn.CloseWithError(code, message); err != nil {
			t.logger.Error("failed to close connection", "error", err)
		}
	})
}

func (t *Transport) Close() error {
	// TODO
	return nil
}
