package moqtransport

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
)

type Transport struct {
	logger *slog.Logger

	session *Session

	Conn Connection

	InitialMaxSubscribeID uint64
	DatagramsDisabled     bool

	Handler Handler

	Qlogger *qlog.Logger

	ctx       context.Context
	cancelCtx context.CancelCauseFunc

	closeOnce sync.Once
}

func (t *Transport) NewSession(ctx context.Context) (*Session, error) {
	t.logger = defaultLogger.With("perspective", t.Conn.Perspective())
	t.logger.Info("NewSession")

	controlStream := newControlStream(t, t.Qlogger)

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

func (t *Transport) handleSubscription(m *Message) {
	lt := newLocalTrack(t.Conn, m.SubscribeID, m.TrackAlias, func(code, count uint64, reason string) error {
		return t.session.subscriptionDone(m.SubscribeID, code, count, reason)
	}, t.Qlogger)

	if err := t.session.addLocalTrack(lt); err != nil {
		if err == errMaxSubscribeIDViolated || err == errDuplicateSubscribeID {
			t.handleProtocolViolation(err)
			return
		}
		if rejectErr := t.session.rejectSubscription(m.SubscribeID, ErrorCodeSubscribeInternal, ""); rejectErr != nil {
			t.logger.Error("failed to add localtrack and failed to reject subscription", "error", err, "rejectErr", rejectErr)
			// TODO: Close conn?
		}
		return
	}
	srw := &subscriptionResponseWriter{
		id:         m.SubscribeID,
		trackAlias: m.TrackAlias,
		session:    t.session,
		localTrack: lt,
		handled:    false,
	}
	t.Handler.Handle(srw, m)
	if !srw.handled {
		if err := srw.Reject(0, "unhandled subscription"); err != nil {
			t.logger.Error("failed to reject subscription", "error", err)
		}
	}

}

func (t *Transport) handleFetch(m *Message) {
	lt := newLocalTrack(t.Conn, m.SubscribeID, m.TrackAlias, nil, t.Qlogger)
	if err := t.session.addLocalTrack(lt); err != nil {
		if err == errMaxSubscribeIDViolated || err == errDuplicateSubscribeID {
			t.handleProtocolViolation(err)
			return
		}
		if rejectErr := t.session.rejectFetch(m.SubscribeID, ErrorCodeSubscribeInternal, ""); rejectErr != nil {
			t.logger.Error("failed to add localtrack and failed to reject fetch", "error", err, "rejectErr", rejectErr)
			// TODO: Close conn?
		}
		return
	}
	frw := &fetchResponseWriter{
		id:         m.SubscribeID,
		session:    t.session,
		localTrack: lt,
		handled:    false,
	}
	t.Handler.Handle(frw, m)
	if !frw.handled {
		if err := frw.Reject(0, "unhandled fetch"); err != nil {
			t.logger.Error("failed to reject fetch", "error", err)
		}
	}
}

func (t *Transport) handle(m *Message) {
	if t.Handler != nil {
		switch m.Method {
		case MessageSubscribe:
			t.handleSubscription(m)
		case MessageFetch:
			t.handleFetch(m)
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
		maxSubscribeID:                           t.InitialMaxSubscribeID,
		outgoingAnnouncements:                    newAnnouncementMap(),
		incomingAnnouncements:                    newAnnouncementMap(),
		pendingOutgointAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		pendingIncomingAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		highestSubscribesBlocked:                 atomic.Uint64{},
		remoteTracks:                             newRemoteTrackMap(0),
		localTracks:                              newLocalTrackMap(),
		outgoingTrackStatusRequests:              newTrackStatusRequestMap(),
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
			parser, err := wire.NewObjectStreamParser(stream, stream.StreamID(), t.Qlogger)
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
			msg := new(wire.ObjectMessage)
			_, err := msg.ParseDatagram(dgram)
			if err != nil {
				t.logger.Error("failed to parse datagram object", "error", err)
				return
			}
			if t.Qlogger != nil {
				eth := slices.Collect(slices.Map(
					msg.ObjectHeaderExtensions,
					func(e wire.ObjectHeaderExtension) moqt.ExtensionHeader {
						return moqt.ExtensionHeader{
							HeaderType:   0, // TODO
							HeaderValue:  0, // TODO
							HeaderLength: 0, // TODO
							Payload:      qlog.RawInfo{},
						}
					}),
				)
				t.Qlogger.Log(moqt.ObjectDatagramEvent{
					EventName:              moqt.ObjectDatagramEventparsed,
					TrackAlias:             msg.TrackAlias,
					GroupID:                msg.GroupID,
					ObjectID:               msg.ObjectID,
					PublisherPriority:      msg.PublisherPriority,
					ExtensionHeadersLength: uint64(len(msg.ObjectHeaderExtensions)),
					ExtensionHeaders:       eth,
					ObjectStatus:           uint64(msg.ObjectStatus),
					Payload: qlog.RawInfo{
						Length:        uint64(len(msg.ObjectPayload)),
						PayloadLength: uint64(len(msg.ObjectPayload)),
						Data:          msg.ObjectPayload,
					},
				})
			}
			if err := t.session.receiveDatagram(msg); err != nil {
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
