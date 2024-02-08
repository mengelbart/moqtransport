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
	"github.com/quic-go/webtransport-go"
)

type messageHandler func(message) error

type controlStreamHandler interface {
	io.Reader
	send(message) error
	readMessages(handler messageHandler)
}

type defaultCtrlStreamHandler struct {
	logger *slog.Logger
	stream stream
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
			panic(fmt.Sprintf("TODO: %v", err))
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

func sendOnStream(stream sendStream, msg message) error {
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
	ctx    context.Context
	logger *slog.Logger

	conn connection
	cms  controlStreamHandler

	enableDatagrams bool

	subscriptionCh chan *SendSubscription
	announcementCh chan *Announcement

	activeSendSubscriptionsLock sync.RWMutex
	activeSendSubscriptions     map[uint64]*SendSubscription

	activeReceiveSubscriptionsLock sync.RWMutex
	activeReceiveSubscriptions     map[uint64]*ReceiveSubscription

	pendingSubscriptionsLock sync.RWMutex
	pendingSubscriptions     map[uint64]*ReceiveSubscription

	pendingAnnouncementsLock sync.RWMutex
	pendingAnnouncements     map[string]*announcement
}

func NewWebtransportServerSession(ctx context.Context, conn *webtransport.Session, enableDatagrams bool) (*Session, error) {
	return newServerSession(ctx, &webTransportConn{session: conn}, enableDatagrams)
}

func newClientSession(ctx context.Context, conn connection, clientRole Role, enableDatagrams bool) (*Session, error) {
	ctrlStream, err := conn.OpenStreamSync(ctx)
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
		return nil, &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected first message on control stream",
		}
	}
	if !slices.Contains(csm.SupportedVersions, ssm.SelectedVersion) {
		return nil, errUnsupportedVersion
	}
	s, err := newSession(ctx, conn, ctrlStreamHandler, enableDatagrams), nil
	if err != nil {
		return nil, err
	}
	s.run()
	return s, nil
}

func newServerSession(ctx context.Context, conn connection, enableDatagrams bool) (*Session, error) {
	ctrlStream, err := conn.AcceptStream(ctx)
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
		return nil, &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected first message on control stream",
		}
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, CURRENT_VERSION) {
		return nil, errUnsupportedVersion
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		return nil, &moqError{
			code:    genericErrorErrorCode,
			message: "missing role parameter",
		}
	}
	// TODO: save role parameter
	ssm := &serverSetupMessage{
		SelectedVersion: CURRENT_VERSION,
		SetupParameters: map[uint64]parameter{},
	}
	if err = ctrlStreamHandler.send(ssm); err != nil {
		return nil, fmt.Errorf("sending message on control stream failed: %w", err)
	}
	s, err := newSession(ctx, conn, ctrlStreamHandler, enableDatagrams), nil
	if err != nil {
		return nil, err
	}
	s.run()
	return s, nil
}

func newSession(ctx context.Context, conn connection, cms controlStreamHandler, enableDatagrams bool) *Session {
	s := &Session{
		ctx:                            ctx,
		logger:                         defaultLogger.WithGroup("MOQ_SESSION"),
		conn:                           conn,
		cms:                            cms,
		enableDatagrams:                enableDatagrams,
		subscriptionCh:                 make(chan *SendSubscription),
		announcementCh:                 make(chan *Announcement),
		activeSendSubscriptionsLock:    sync.RWMutex{},
		activeSendSubscriptions:        map[uint64]*SendSubscription{},
		activeReceiveSubscriptionsLock: sync.RWMutex{},
		activeReceiveSubscriptions:     map[uint64]*ReceiveSubscription{},
		pendingSubscriptionsLock:       sync.RWMutex{},
		pendingSubscriptions:           map[uint64]*ReceiveSubscription{},
		pendingAnnouncementsLock:       sync.RWMutex{},
		pendingAnnouncements:           map[string]*announcement{},
	}
	return s
}

func (s *Session) run() {
	go s.acceptUnidirectionalStreams(s.ctx)
	if s.enableDatagrams {
		go s.acceptDatagrams(s.ctx)
	}
	go s.cms.readMessages(s.handleControlMessage)
}

func (s *Session) acceptUnidirectionalStreams(ctx context.Context) {
	for {
		stream, err := s.conn.AcceptUniStream(ctx)
		if err != nil {
			s.logger.Error("failed to accept uni stream", "error", err)
			return
		}
		go s.handleIncomingUniStream(stream)
	}
}

func (s *Session) handleIncomingUniStream(stream receiveStream) {
	p := newParser(quicvarint.NewReader(stream))
	msg, err := p.parse()
	if err != nil {
		s.logger.Error("failed to parse message", "error", err)
		return
	}
	switch h := msg.(type) {
	case *objectMessage:
		// TODO: Only parse header and then delegate reading payload to ReceiveSubscription
		if err := s.handleObjectMessage(msg); err != nil {
			panic(err) // TODO
		}
		return
	case *streamHeaderTrackMessage:
		s.activeReceiveSubscriptionsLock.RLock()
		sub, ok := s.activeReceiveSubscriptions[h.SubscribeID]
		s.activeReceiveSubscriptionsLock.RUnlock()
		if !ok {
			s.logger.Warn("got stream header track message for unknown subscription")
			panic("TODO")
		}
		sub.readTrackHeaderStream(stream)
	case *streamHeaderGroupMessage:
		s.activeReceiveSubscriptionsLock.RLock()
		sub, ok := s.activeReceiveSubscriptions[h.SubscribeID]
		s.activeReceiveSubscriptionsLock.RUnlock()
		if !ok {
			s.logger.Warn("got stream header track message for unknown subscription")
			panic("TODO")
		}
		sub.readGroupHeaderStream(stream)
	}
}

func (s *Session) acceptDatagrams(ctx context.Context) {
	for {
		dgram, err := s.conn.ReceiveMessage(ctx)
		if err != nil {
			s.logger.Error("failed to receive datagram", "error", err)
			return
		}
		go s.readMessages(bytes.NewReader(dgram), s.handleObjectMessage)
	}
}

func (s *Session) readMessages(r messageReader, handle messageHandler) {
	msgParser := newParser(r)
	for {
		msg, err := msgParser.parse()
		if err != nil {
			if err == io.EOF {
				return
			}
			s.logger.Error("TODO", "error", err)
			return
		}
		if err = handle(msg); err != nil {
			panic(fmt.Sprintf("TODO: %v", err))
		}
	}
}

func (s *Session) handleControlMessage(msg message) error {
	var err error
	switch m := msg.(type) {
	case *objectMessage:
		return &moqError{
			code:    protocolViolationErrorCode,
			message: "received object message on control stream",
		}
	case *subscribeMessage:
		err = s.cms.send(s.handleSubscribeRequest(m))
	case *subscribeOkMessage:
		err = s.handleSubscriptionResponse(m)
	case *subscribeErrorMessage:
		err = s.handleSubscriptionResponse(m)
	case *subscribeFinMessage:
		panic("TODO")
	case *subscribeRstMessage:
		panic("TODO")
	case *unsubscribeMessage:
		panic("TODO")
	case *announceMessage:
		err = s.cms.send(s.handleAnnounceMessage(m))
	case *announceOkMessage:
		err = s.handleAnnouncementResponse(m)
	case *announceErrorMessage:
		err = s.handleAnnouncementResponse(m)
	case *goAwayMessage:
		panic("TODO")
	default:
		return &moqError{
			code:    genericErrorErrorCode,
			message: "received unexpected message type on control stream",
		}
	}
	return err
}

func (s *Session) handleSubscriptionResponse(msg subscribeIDer) error {
	s.pendingSubscriptionsLock.RLock()
	sub, ok := s.pendingSubscriptions[msg.subscribeID()]
	s.pendingSubscriptionsLock.RUnlock()
	if !ok {
		return &moqError{
			code:    genericErrorErrorCode,
			message: "received subscription response message to an unknown subscription",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case sub.responseCh <- msg:
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
	return nil
}

func (s *Session) handleAnnouncementResponse(msg trackNamespacer) error {
	s.pendingAnnouncementsLock.RLock()
	a, ok := s.pendingAnnouncements[msg.trackNamespace()]
	s.pendingAnnouncementsLock.RUnlock()
	if !ok {
		return &moqError{
			code:    genericErrorErrorCode,
			message: "received announcement response message to an unknown announcement",
		}
	}
	// TODO: Run a goroutine to avoid blocking here?
	select {
	case a.responseCh <- msg:
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
	return nil
}

func (s *Session) handleSubscribeRequest(msg *subscribeMessage) message {
	sub := &SendSubscription{
		lock:        sync.RWMutex{},
		responseCh:  make(chan error),
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
	select {
	case <-s.ctx.Done():
		return &subscribeErrorMessage{
			SubscribeID:  msg.SubscribeID,
			ErrorCode:    0,
			ReasonPhrase: "session closed",
			TrackAlias:   msg.TrackAlias,
		}
	case s.subscriptionCh <- sub:
	}
	select {
	case <-s.ctx.Done():
		close(sub.closeCh)
	case err := <-sub.responseCh:
		if err != nil {
			return &subscribeErrorMessage{
				SubscribeID:  msg.SubscribeID,
				ErrorCode:    0,
				ReasonPhrase: fmt.Sprintf("subscription rejected: %v", err),
				TrackAlias:   msg.TrackAlias,
			}
		}
	}
	return &subscribeOkMessage{
		SubscribeID: msg.SubscribeID,
		Expires:     sub.expires,
	}
}

func (s *Session) handleAnnounceMessage(msg *announceMessage) message {
	a := &Announcement{
		errorCh:    make(chan error),
		closeCh:    make(chan struct{}),
		namespace:  msg.TrackNamespace,
		parameters: msg.TrackRequestParameters,
	}
	select {
	case <-s.ctx.Done():
		return &announceErrorMessage{
			TrackNamespace: msg.TrackNamespace,
			ErrorCode:      0, // TODO: Set correct code?
			ReasonPhrase:   "session closed",
		}
	case s.announcementCh <- a:
	}
	select {
	case <-s.ctx.Done():
		close(a.closeCh)
	case err := <-a.errorCh:
		if err != nil {
			return &announceErrorMessage{
				TrackNamespace: a.namespace,
				ErrorCode:      0, // TODO: Implement a custom error type including the code?
				ReasonPhrase:   fmt.Sprintf("announcement rejected: %v", err),
			}
		}
	}
	return &announceOkMessage{
		TrackNamespace: a.namespace,
	}
}

func (s *Session) handleObjectMessage(m message) error {
	o, ok := m.(*objectMessage)
	if !ok {
		return &moqError{
			code:    protocolViolationErrorCode,
			message: "received unexpected control message on object stream",
		}
	}
	s.activeReceiveSubscriptionsLock.RLock()
	sub, ok := s.activeReceiveSubscriptions[o.SubscribeID]
	s.activeReceiveSubscriptionsLock.RUnlock()
	if ok {
		_, err := sub.push(o)
		return err
	}
	s.logger.Warn("dropping object message for unknown track")
	return nil
}

func (s *Session) ReadSubscription(ctx context.Context) (*SendSubscription, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("session closed") // TODO: Better error message including a reason?
	case s := <-s.subscriptionCh:
		return s, nil
	}
}

func (s *Session) ReadAnnouncement(ctx context.Context) (*Announcement, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("session closed") // TODO: Better error message including a reason?
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
	sub := newReceiveSubscription()
	s.pendingSubscriptionsLock.Lock()
	s.pendingSubscriptions[sm.subscribeID()] = sub
	s.pendingSubscriptionsLock.Unlock()
	if err := s.cms.send(sm); err != nil {
		return nil, err
	}
	var resp subscribeIDer
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case resp = <-sub.responseCh:
	}
	if resp.subscribeID() != sm.SubscribeID {
		s.logger.Error("internal error: received response message for wrong subscription ID.", "expected_id", sm.SubscribeID, "repsonse_id", resp.subscribeID())
		return nil, &moqError{
			code:    genericErrorErrorCode,
			message: "internal error",
		}
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		s.activeReceiveSubscriptionsLock.Lock()
		s.activeReceiveSubscriptions[v.SubscribeID] = sub
		s.activeReceiveSubscriptionsLock.Unlock()
		return sub, nil
	case *subscribeErrorMessage:
		return nil, errors.New(v.ReasonPhrase)
	}
	return nil, &moqError{
		code:    genericErrorErrorCode,
		message: "received unexpected response message type to subscribeRequestMessage",
	}
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
	s.pendingAnnouncementsLock.Lock()
	s.pendingAnnouncements[am.TrackNamespace] = t
	s.pendingAnnouncementsLock.Unlock()
	if err := s.cms.send(am); err != nil {
		return err
	}
	var resp trackNamespacer
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	case resp = <-responseCh:
	}
	if resp.trackNamespace() != am.TrackNamespace {
		s.logger.Error("internal error: received response message for wrong announce track namespace.", "expected_track_namespace", am.TrackNamespace, "response_track_namespace", resp.trackNamespace())
		return &moqError{
			code:    genericErrorErrorCode,
			message: "internal error",
		}
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		return nil
	case *announceErrorMessage:
		return errors.New(v.ReasonPhrase)
	}
	return &moqError{
		code:    genericErrorErrorCode,
		message: "received unexpected response message type to announceMessage",
	}
}

func (s *Session) CloseWithError(code uint64, msg string) {

}
