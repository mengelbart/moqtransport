package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errClosed             = errors.New("session closed")
	errUnsupportedVersion = errors.New("unsupported version")
)

type messageHandler func(message) error

type controlStreamHandler interface {
	io.Reader
	send(message) error
	readMessages(handler messageHandler)
}

type defaultCtrlStreamHandler struct {
	logger *slog.Logger
	stream Stream
	parser parser
}

func (h *defaultCtrlStreamHandler) readNext() (message, error) {
	msg, err := h.parser.parse()
	h.logger.Info("handling message", "message", msg)
	return msg, err
}

func (h *defaultCtrlStreamHandler) readMessages(handler messageHandler) {
	for {
		msg, err := h.readNext()
		if err != nil {
			if err == io.EOF {
				return
			}
			h.logger.Error("TODO", "error", err)
			return
		}
		if err = handler(msg); err != nil {
			h.logger.Error("TODO", "error", err)
			return
		}
	}
}

func (h *defaultCtrlStreamHandler) Read(buf []byte) (int, error) {
	return h.stream.Read(buf)
}

func (s defaultCtrlStreamHandler) send(msg message) error {
	s.logger.Info("sending message", "message", msg)
	return sendOnStream(s.stream, msg)
}

func sendOnStream(stream SendStream, msg message) error {
	buf := make([]byte, 0, 1500)
	buf = msg.append(buf)
	if _, err := stream.Write(buf); err != nil {
		return err
	}
	return nil
}

type parser interface {
	parse() (message, error)
}

type announcement struct {
	responseCh chan trackNamespacer
}

type subscribeIDer interface {
	message
	subscribeID() uint64
}

type trackNamespacer interface {
	message
	trackNamespace() string
}

type Session struct {
	logger *slog.Logger

	closeOnce sync.Once
	closed    chan struct{}
	conn      Connection
	cms       controlStreamHandler

	enableDatagrams bool

	subscriptionCh chan *SendSubscription
	announcementCh chan *Announcement

	sendSubscriptionsLock sync.RWMutex
	sendSubscriptions     map[uint64]*SendSubscription

	receiveSubscriptionsLock sync.RWMutex
	receiveSubscriptions     map[uint64]*ReceiveSubscription

	announcementsLock sync.RWMutex
	announcements     map[string]*announcement
}

func NewClientSession(conn Connection, clientRole Role, enableDatagrams bool) (*Session, error) {
	ctrlStream, err := conn.OpenStreamSync(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("opening control stream failed: %w", err)
	}
	ctrlStreamHandler := &defaultCtrlStreamHandler{
		logger: defaultLogger.WithGroup("MOQ_CONTROL_STREAM"),
		stream: ctrlStream,
		parser: newParser(quicvarint.NewReader(ctrlStream)),
	}
	csm := &clientSetupMessage{
		SupportedVersions: []version{CURRENT_VERSION},
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: uint64(clientRole),
			},
		},
	}
	if err = ctrlStreamHandler.send(csm); err != nil {
		return nil, fmt.Errorf("sending message on control stream failed: %w", err)
	}
	msg, err := ctrlStreamHandler.readNext()
	if err != nil {
		return nil, fmt.Errorf("parsing message filed: %w", err)
	}
	ssm, ok := msg.(*serverSetupMessage)
	if !ok {
		pe := ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "received unexpected first message on control stream",
		}
		_ = conn.CloseWithError(pe.code, pe.message)
		return nil, pe
	}
	if !slices.Contains(csm.SupportedVersions, ssm.SelectedVersion) {
		return nil, errUnsupportedVersion
	}
	l := defaultLogger.WithGroup("MOQ_CLIENT_SESSION")
	s, err := newSession(conn, ctrlStreamHandler, enableDatagrams, l), nil
	if err != nil {
		return nil, err
	}
	s.run()
	return s, nil
}

func NewServerSession(conn Connection, enableDatagrams bool) (*Session, error) {
	ctrlStream, err := conn.AcceptStream(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("accepting control stream failed: %w", err)
	}
	ctrlStreamHandler := &defaultCtrlStreamHandler{
		logger: defaultLogger.WithGroup("MOQ_CONTROL_STREAM"),
		stream: ctrlStream,
		parser: newParser(quicvarint.NewReader(ctrlStream)),
	}
	m, err := ctrlStreamHandler.readNext()
	if err != nil {
		return nil, fmt.Errorf("parsing message failed: %w", err)
	}
	msg, ok := m.(*clientSetupMessage)
	if !ok {
		pe := ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "received unexpected first message on control stream",
		}
		_ = conn.CloseWithError(pe.code, pe.message)
		return nil, pe
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, CURRENT_VERSION) {
		return nil, errUnsupportedVersion
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		pe := ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "missing role parameter",
		}
		_ = conn.CloseWithError(pe.code, pe.message)
		return nil, pe
	}
	// TODO: save role parameter
	ssm := &serverSetupMessage{
		SelectedVersion: CURRENT_VERSION,
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: uint64(IngestionDeliveryRole),
			},
		},
	}
	if err = ctrlStreamHandler.send(ssm); err != nil {
		return nil, fmt.Errorf("sending message on control stream failed: %w", err)
	}
	l := defaultLogger.WithGroup("MOQ_SERVER_SESSION")
	s, err := newSession(conn, ctrlStreamHandler, enableDatagrams, l), nil
	if err != nil {
		return nil, err
	}
	s.run()
	return s, nil
}

func newSession(conn Connection, cms controlStreamHandler, enableDatagrams bool, logger *slog.Logger) *Session {
	s := &Session{
		logger:                   logger,
		closed:                   make(chan struct{}),
		conn:                     conn,
		cms:                      cms,
		enableDatagrams:          enableDatagrams,
		subscriptionCh:           make(chan *SendSubscription),
		announcementCh:           make(chan *Announcement),
		sendSubscriptionsLock:    sync.RWMutex{},
		sendSubscriptions:        map[uint64]*SendSubscription{},
		receiveSubscriptionsLock: sync.RWMutex{},
		receiveSubscriptions:     map[uint64]*ReceiveSubscription{},
		announcementsLock:        sync.RWMutex{},
		announcements:            map[string]*announcement{},
	}
	return s
}

func (s *Session) run() {
	go s.acceptUnidirectionalStreams()
	if s.enableDatagrams {
		go s.acceptDatagrams()
	}
	go s.cms.readMessages(s.handleControlMessage)
}

func (s *Session) acceptUnidirectionalStream() (ReceiveStream, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.closed:
			cancel()
		case <-ctx.Done():
		}
	}()
	return s.conn.AcceptUniStream(ctx)
}

func (s *Session) acceptUnidirectionalStreams() {
	for {
		stream, err := s.acceptUnidirectionalStream()
		if err != nil {
			s.logger.Error("failed to accept uni stream", "error", err)
			s.peerClosed()
			return
		}
		go s.handleIncomingUniStream(stream)
	}
}

func (s *Session) handleIncomingUniStream(stream ReceiveStream) {
	p := newParser(quicvarint.NewReader(stream))
	msg, err := p.parse()
	if err != nil {
		s.logger.Error("failed to parse message", "error", err)
		return
	}
	switch h := msg.(type) {
	case *objectMessage:
		s.receiveSubscriptionsLock.RLock()
		sub, ok := s.receiveSubscriptions[h.SubscribeID]
		s.receiveSubscriptionsLock.RUnlock()
		if !ok {
			s.logger.Warn("got object for unknown subscribe ID")
			return
		}
		if _, err := sub.push(h); err != nil {
			panic(err)
		}
	case *streamHeaderTrackMessage:
		s.receiveSubscriptionsLock.RLock()
		sub, ok := s.receiveSubscriptions[h.SubscribeID]
		s.receiveSubscriptionsLock.RUnlock()
		if !ok {
			s.logger.Warn("got stream header track message for unknown subscription")
			return
		}
		sub.readTrackHeaderStream(stream)
	case *streamHeaderGroupMessage:
		s.receiveSubscriptionsLock.RLock()
		sub, ok := s.receiveSubscriptions[h.SubscribeID]
		s.receiveSubscriptionsLock.RUnlock()
		if !ok {
			s.logger.Warn("got stream header track message for unknown subscription")
			return
		}
		sub.readGroupHeaderStream(stream)
	}
}

func (s *Session) acceptDatagram() ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.closed:
			cancel()
		case <-ctx.Done():
		}
	}()
	return s.conn.ReceiveDatagram(ctx)
}

func (s *Session) acceptDatagrams() {
	for {
		dgram, err := s.acceptDatagram()
		if err != nil {
			s.logger.Error("failed to receive datagram", "error", err)
			s.peerClosed()
			return
		}
		go s.readObjectMessages(bytes.NewReader(dgram))
	}
}

func (s *Session) readObjectMessages(r messageReader) {
	msgParser := newParser(r)
	for {
		msg, err := msgParser.parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			s.logger.Error("failed to parse message", "error", err)
			pe := &ProtocolError{
				code:    ErrorCodeProtocolViolation,
				message: "invalid message format",
			}
			_ = s.conn.CloseWithError(pe.code, pe.message)
			return
		}
		o, ok := msg.(*objectMessage)
		if !ok {
			pe := &ProtocolError{
				code:    ErrorCodeProtocolViolation,
				message: "received unexpected control message on object stream or datagram",
			}
			// TODO: Set error on session to surface to application?
			_ = s.conn.CloseWithError(pe.code, pe.message)
			return
		}
		if err = s.handleObjectMessage(o); err != nil {
			s.logger.Info("failed to handle message", "error", err)
			return
		}
	}
}

func (s *Session) handleControlMessage(msg message) error {
	var err error
	switch m := msg.(type) {
	case *objectMessage:
		pe := &ProtocolError{
			code:    ErrorCodeProtocolViolation,
			message: "received object message on control stream",
		}
		return pe
	case *subscribeMessage:
		r := s.handleSubscribe(m)
		if r != nil {
			err = s.cms.send(r)
		}
	case *subscribeOkMessage:
		err = s.handleSubscriptionResponse(m)
	case *subscribeErrorMessage:
		err = s.handleSubscriptionResponse(m)
	case *subscribeDoneMessage:
		panic("TODO")
	case *unsubscribeMessage:
		s.handleUnsubscribe(m)
	case *announceMessage:
		r := s.handleAnnounceMessage(m)
		if r != nil {
			err = s.cms.send(r)
		}
	case *announceOkMessage:
		err = s.handleAnnouncementResponse(m)
	case *announceErrorMessage:
		err = s.handleAnnouncementResponse(m)
	case *goAwayMessage:
		panic("TODO")
	default:
		return &ProtocolError{
			code:    ErrorCodeInternal,
			message: "received unexpected message type on control stream",
		}
	}
	return err
}

func (s *Session) handleSubscriptionResponse(msg subscribeIDer) error {
	s.receiveSubscriptionsLock.RLock()
	sub, ok := s.receiveSubscriptions[msg.subscribeID()]
	s.receiveSubscriptionsLock.RUnlock()
	if !ok {
		return &ProtocolError{
			code:    ErrorCodeInternal,
			message: "received subscription response message to an unknown subscription",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case sub.responseCh <- msg:
	case <-s.closed:
		return errClosed
	}
	return nil
}

func (s *Session) handleAnnouncementResponse(msg trackNamespacer) error {
	s.announcementsLock.RLock()
	a, ok := s.announcements[msg.trackNamespace()]
	s.announcementsLock.RUnlock()
	if !ok {
		return &ProtocolError{
			code:    ErrorCodeInternal,
			message: "received announcement response message to an unknown announcement",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case a.responseCh <- msg:
	case <-s.closed:
		return errClosed
	}
	return nil
}

func (s *Session) handleSubscribe(msg *subscribeMessage) message {
	sub := &SendSubscription{
		lock:        sync.RWMutex{},
		responseCh:  make(chan *subscribeError),
		closeCh:     make(chan struct{}),
		expires:     0,
		conn:        s.conn,
		subscribeID: msg.SubscribeID,
		trackAlias:  msg.TrackAlias,
		namespace:   msg.TrackNamespace,
		trackname:   msg.TrackName,
		startGroup:  msg.StartGroup,
		startObject: msg.StartObject,
		endGroup:    msg.EndGroup,
		endObject:   msg.EndObject,
		parameters:  msg.Parameters,
	}
	s.sendSubscriptionsLock.Lock()
	s.sendSubscriptions[sub.subscribeID] = sub
	s.sendSubscriptionsLock.Unlock()
	select {
	case <-s.closed:
		return nil
	case s.subscriptionCh <- sub:
	}
	select {
	case <-s.closed:
		close(sub.closeCh)
		return nil
	case err := <-sub.responseCh:
		if err != nil {
			return &subscribeErrorMessage{
				SubscribeID:  msg.SubscribeID,
				ErrorCode:    err.code,
				ReasonPhrase: err.reason,
				TrackAlias:   msg.TrackAlias,
			}
		}
	}
	return &subscribeOkMessage{
		SubscribeID: msg.SubscribeID,
		Expires:     sub.expires,
	}
}

func (s *Session) handleUnsubscribe(msg *unsubscribeMessage) {
	s.sendSubscriptionsLock.Lock()
	sub, ok := s.sendSubscriptions[msg.SubscribeID]
	if !ok {
		s.logger.Info("got unsubscribe for unknown subscribe ID", "id", msg.SubscribeID)
		return
	}
	delete(s.sendSubscriptions, msg.SubscribeID)
	s.sendSubscriptionsLock.Unlock()
	sub.unsubscribe()
}

func (s *Session) handleAnnounceMessage(msg *announceMessage) message {
	a := &Announcement{
		errorCh:    make(chan *announcementError),
		closeCh:    make(chan struct{}),
		namespace:  msg.TrackNamespace,
		parameters: msg.TrackRequestParameters,
	}
	select {
	case <-s.closed:
		return nil
	case s.announcementCh <- a:
	}
	select {
	case <-s.closed:
		close(a.closeCh)
		return nil
	case err := <-a.errorCh:
		if err != nil {
			return &announceErrorMessage{
				TrackNamespace: a.namespace,
				ErrorCode:      err.code,
				ReasonPhrase:   err.reason,
			}
		}
	}
	return &announceOkMessage{
		TrackNamespace: a.namespace,
	}
}

func (s *Session) handleObjectMessage(o *objectMessage) error {
	s.receiveSubscriptionsLock.RLock()
	sub, ok := s.receiveSubscriptions[o.SubscribeID]
	s.receiveSubscriptionsLock.RUnlock()
	if ok {
		_, err := sub.push(o)
		return err
	}
	s.logger.Warn("dropping object message for unknown track")
	return nil
}

func (s *Session) unsubscribe(id uint64) error {
	return s.cms.send(&unsubscribeMessage{
		SubscribeID: id,
	})
}

func (s *Session) peerClosed() {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
}

func (s *Session) CloseWithError(code uint64, msg string) error {
	s.peerClosed()
	return s.conn.CloseWithError(code, msg)
}

func (s *Session) Close() error {
	return s.CloseWithError(0, "")
}

func (s *Session) ReadSubscription(ctx context.Context) (*SendSubscription, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, errClosed // TODO: Better error message including a reason?
	case s := <-s.subscriptionCh:
		return s, nil
	}
}

func (s *Session) ReadAnnouncement(ctx context.Context) (*Announcement, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, errClosed // TODO: Better error message including a reason?
	case a := <-s.announcementCh:
		return a, nil
	}
}

func (s *Session) Subscribe(ctx context.Context, subscribeID, trackAlias uint64, namespace, trackname, auth string) (*ReceiveSubscription, error) {
	sm := &subscribeMessage{
		SubscribeID:    subscribeID,
		TrackAlias:     trackAlias,
		TrackNamespace: namespace,
		TrackName:      trackname,
		StartGroup:     Location{},
		StartObject:    Location{},
		EndGroup:       Location{},
		EndObject:      Location{},
		Parameters:     map[uint64]parameter{},
	}
	if len(auth) > 0 {
		sm.Parameters[authorizationParameterKey] = stringParameter{
			k: authorizationParameterKey,
			v: auth,
		}
	}
	sub := newReceiveSubscription(sm.SubscribeID, s)
	s.receiveSubscriptionsLock.Lock()
	s.receiveSubscriptions[sm.subscribeID()] = sub
	s.receiveSubscriptionsLock.Unlock()
	if err := s.cms.send(sm); err != nil {
		return nil, err
	}
	var resp subscribeIDer
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, errClosed
	case resp = <-sub.responseCh:
	}
	if resp.subscribeID() != sm.SubscribeID {
		// Should never happen, because messages are routed based on subscribe
		// ID. Wrong IDs would thus never end up here.
		s.logger.Error("internal error: received response message for wrong subscription ID", "expected_id", sm.SubscribeID, "repsonse_id", resp.subscribeID())
		return nil, errors.New("internal error: received response message for wrong subscription ID")
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		return sub, nil
	case *subscribeErrorMessage:
		s.receiveSubscriptionsLock.Lock()
		delete(s.receiveSubscriptions, sm.subscribeID())
		s.receiveSubscriptionsLock.Unlock()
		return nil, ApplicationError{
			code:   v.ErrorCode,
			mesage: v.ReasonPhrase,
		}
	}
	// Should never happen, because only subscribeMessage, subscribeOkMessage
	// and susbcribeErrorMessage implement the SubscribeIDer interface and
	// subscribeMessages should not be routed to this method.
	return nil, errors.New("received unexpected response message type to subscribeRequestMessage")
}

func (s *Session) Announce(ctx context.Context, namespace string) error {
	if len(namespace) == 0 {
		return errors.New("invalid track namespace")
	}
	am := &announceMessage{
		TrackNamespace:         namespace,
		TrackRequestParameters: map[uint64]parameter{},
	}
	responseCh := make(chan trackNamespacer)
	t := &announcement{
		responseCh: responseCh,
	}
	s.announcementsLock.Lock()
	s.announcements[am.TrackNamespace] = t
	s.announcementsLock.Unlock()
	if err := s.cms.send(am); err != nil {
		return err
	}
	var resp trackNamespacer
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.closed:
		return errClosed
	case resp = <-responseCh:
	}
	if resp.trackNamespace() != am.TrackNamespace {
		// Should never happen, because messages are routed based on trackname.
		// Wrong tracknames would thus never end up here.
		s.logger.Error("internal error: received response message for wrong announce track namespace", "expected_track_namespace", am.TrackNamespace, "response_track_namespace", resp.trackNamespace())
		return errors.New("internal error: received response message for wrong announce track namespace")
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		return nil
	case *announceErrorMessage:
		return ApplicationError{
			code:   v.ErrorCode,
			mesage: v.ReasonPhrase,
		}
	}
	// Should never happen, because only announceMessage, announceOkMessage
	// and announceErrorMessage implement the trackNamespacer interface and
	// announceMessages should not be routed to this method.
	return errors.New("received unexpected response message type to announceMessage")
}
