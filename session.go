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

	sendSubscriptions    *subscriptionMap[*SendSubscription]
	receiveSubscriptions *subscriptionMap[*ReceiveSubscription]
	localAnnouncements   *announcementMap
	remoteAnnouncements  *announcementMap
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
		logger:               logger,
		closed:               make(chan struct{}),
		conn:                 conn,
		cms:                  cms,
		enableDatagrams:      enableDatagrams,
		sendSubscriptions:    newSubscriptionMap[*SendSubscription](),
		receiveSubscriptions: newSubscriptionMap[*ReceiveSubscription](),
		localAnnouncements:   newAnnouncementMap(),
		remoteAnnouncements:  newAnnouncementMap(),
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
		sub, ok := s.receiveSubscriptions.get(h.SubscribeID)
		if !ok {
			s.logger.Warn("got object for unknown subscribe ID")
			return
		}
		if _, err := sub.push(h); err != nil {
			panic(err)
		}
	case *streamHeaderTrackMessage:
		sub, ok := s.receiveSubscriptions.get(h.SubscribeID)
		if !ok {
			s.logger.Warn("got stream header track message for unknown subscription")
			return
		}
		sub.readTrackHeaderStream(stream)
	case *streamHeaderGroupMessage:
		sub, ok := s.receiveSubscriptions.get(h.SubscribeID)
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
	switch m := msg.(type) {
	case *subscribeMessage:
		return s.handleSubscribe(m)
	case *subscribeOkMessage:
		return s.handleSubscriptionResponse(m)
	case *subscribeErrorMessage:
		return s.handleSubscriptionResponse(m)
	case *subscribeDoneMessage:
		return s.handleSubscribeDone(m)
	case *unsubscribeMessage:
		return s.handleUnsubscribe(m)
	case *announceMessage:
		return s.handleAnnounceMessage(m)
	case *announceOkMessage:
		return s.handleAnnouncementResponse(m)
	case *announceErrorMessage:
		return s.handleAnnouncementResponse(m)
	case *goAwayMessage:
		panic("TODO")
	}
	return &ProtocolError{
		code:    ErrorCodeInternal,
		message: "received unexpected message type on control stream",
	}
}

func (s *Session) handleSubscriptionResponse(msg subscribeIDer) error {
	sub, ok := s.receiveSubscriptions.get(msg.subscribeID())
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
	a, ok := s.localAnnouncements.get(msg.trackNamespace())
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

func (s *Session) handleSubscribe(msg *subscribeMessage) error {
	sub := &SendSubscription{
		lock:        sync.RWMutex{},
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
	return s.sendSubscriptions.add(sub.subscribeID, sub)
}

func (s *Session) handleUnsubscribe(msg *unsubscribeMessage) error {
	if err := s.sendSubscriptions.delete(msg.SubscribeID); err != nil {
		return err
	}
	return s.cms.send(&subscribeDoneMessage{
		SusbcribeID:   msg.SubscribeID,
		StatusCode:    0,
		ReasonPhrase:  "unsubscribed",
		ContentExists: false,
		FinalGroup:    0,
		FinalObject:   0,
	})
}

func (s *Session) handleSubscribeDone(msg *subscribeDoneMessage) error {
	return s.receiveSubscriptions.delete(msg.SusbcribeID)
}

func (s *Session) handleAnnounceMessage(msg *announceMessage) error {
	a := &Announcement{
		responseCh: make(chan trackNamespacer),
		namespace:  msg.TrackNamespace,
		parameters: msg.TrackRequestParameters,
	}
	return s.remoteAnnouncements.add(a.namespace, a)
}

func (s *Session) handleObjectMessage(o *objectMessage) error {
	sub, ok := s.receiveSubscriptions.get(o.SubscribeID)
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

// TODO: Acceptor func should not pass the complete subscription object but only
// the relevant header info
func (s *Session) ReadSubscription(ctx context.Context, accept func(*SendSubscription) error) (*SendSubscription, error) {
	sub, err := s.sendSubscriptions.getNext(ctx)
	if err != nil {
		return nil, err
	}
	if err = accept(sub); err != nil {
		if err = s.cms.send(&subscribeErrorMessage{
			SubscribeID:  sub.subscribeID,
			ErrorCode:    0,
			ReasonPhrase: err.Error(),
			TrackAlias:   sub.trackAlias,
		}); err != nil {
			panic(err)
		}
		return nil, s.sendSubscriptions.delete(sub.subscribeID)
	}
	err = s.cms.send(&subscribeOkMessage{
		SubscribeID:   sub.subscribeID,
		Expires:       0,     // TODO: Let user set these values?
		ContentExists: false, // TODO: Let user set these values?
		FinalGroup:    0,     // TODO: Let user set these values?
		FinalObject:   0,     // TODO: Let user set these values?
	})
	return sub, err
}

func (s *Session) ReadAnnouncement(ctx context.Context, accept func(*Announcement) error) (*Announcement, error) {
	a, err := s.remoteAnnouncements.getNext(ctx)
	if err != nil {
		return nil, err
	}
	if err = accept(a); err != nil {
		if err = s.cms.send(&announceErrorMessage{
			TrackNamespace: a.namespace,
			ErrorCode:      0,
			ReasonPhrase:   err.Error(),
		}); err != nil {
			panic(err)
		}
		return nil, err
	}
	err = s.cms.send(&announceOkMessage{
		TrackNamespace: a.namespace,
	})
	return a, err
}

func (s *Session) Subscribe(ctx context.Context, subscribeID, trackAlias uint64, namespace, trackname, auth string) (*ReceiveSubscription, error) {
	sm := &subscribeMessage{
		SubscribeID:    subscribeID,
		TrackAlias:     trackAlias,
		TrackNamespace: namespace,
		TrackName:      trackname,
		StartGroup:     Location{LocationModeAbsolute, 0x00},
		StartObject:    Location{LocationModeAbsolute, 0x00},
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
	if err := s.receiveSubscriptions.add(sm.SubscribeID, sub); err != nil {
		return nil, err
	}
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
		_ = s.receiveSubscriptions.delete(sm.SubscribeID)
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
	a := &Announcement{
		responseCh: responseCh,
	}
	if err := s.localAnnouncements.add(am.TrackNamespace, a); err != nil {
		return err
	}
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
