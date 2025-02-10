package moqtransport

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
)

type Transport struct {
	logger *slog.Logger

	session *Session

	Conn Connection

	InitialMaxSubscribeID uint64
	DatagramsDisabled     bool
	Role                  Role

	Handler Handler
}

func (t *Transport) NewSession(ctx context.Context) (*Session, error) {
	t.logger = defaultLogger.With("perspective", t.Conn.Perspective())
	t.logger.Info("NewSession")

	controlStream := newControlStream()
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
			t.Handler.Handle(&subscriptionResponseWriter{
				id:         m.SubscribeID,
				trackAlias: m.TrackAlias,
				session:    t.session,
				localTrack: newLocalTrack(t.Conn, m.SubscribeID, m.TrackAlias, func(code, count uint64, reason string) error {
					return t.session.subscriptionDone(m.SubscribeID, code, count, reason)
				}),
			}, m)
		case MessageFetch:
			t.Handler.Handle(&fetchResponseWriter{
				id:      m.SubscribeID,
				session: t.session,
				localTrack: newLocalTrack(t.Conn, m.SubscribeID, 0, func(code, count uint64, reason string) error {
					return t.session.subscriptionDone(m.SubscribeID, code, count, reason)
				}),
			}, m)
		case MessageAnnounce:
			t.Handler.Handle(&announcementResponseWriter{
				namespace: m.Namespace,
				session:   t.session,
			}, m)
		case MessageSubscribeAnnounces:
			t.Handler.Handle(&announcementSubscriptionResponseWriter{
				prefix:  m.Namespace,
				session: t.session,
			}, m)
		default:
			t.Handler.Handle(nil, m)
		}
	}
	// TODO: Handle unhandled request
}

func (t *Transport) newSession(cs *controlStream) *Session {
	return &Session{
		logger:                                   defaultLogger.With("perspective", t.Conn.Perspective()),
		destroyOnce:                              sync.Once{},
		handshakeDoneCh:                          make(chan struct{}),
		controlMessageSender:                     cs,
		pvh:                                      t,
		handler:                                  t,
		version:                                  0,
		protocol:                                 t.Conn.Protocol(),
		perspective:                              t.Conn.Perspective(),
		localRole:                                t.Role,
		remoteRole:                               0,
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
			// TODO
			return
		}
		go t.session.handleUniStream(stream)
	}
}

func (t *Transport) readDatagrams() {
	for {
		dgram, err := t.Conn.ReceiveDatagram(context.Background())
		if err != nil {
			// TODO
			return
		}
		go t.session.receiveDatagram(dgram)
	}
}

func (t *Transport) handleProtocolViolation(err error) {
	var pv ProtocolError
	code := ErrorCodeInternal
	message := "internal error"
	if errors.As(err, &pv) {
		code = pv.code
		message = pv.message
	}
	if err := t.Conn.CloseWithError(code, message); err != nil {
		t.logger.Error("failed to close connection", "error", err)
	}
}

func (t *Transport) Close() error {
	// TODO
	return nil
}
