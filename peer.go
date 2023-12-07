package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/quic-go/quic-go/quicvarint"
	"golang.org/x/exp/slices"
)

var (
	errUnexpectedMessage     = errors.New("got unexpected message")
	errInvalidTrackNamespace = errors.New("got invalid tracknamespace")
	errUnknownTrack          = errors.New("received object for unknown track")
	errClosed                = errors.New("connection was closed")
	errUnsupportedVersion    = errors.New("unsupported version")
	errMissingRoleParameter  = errors.New("missing role parameter")
)

type parser interface {
	parse() (message, error)
}

type parserFactory interface {
	new(messageReader) parser
}

type parserFactoryFn func(messageReader) parser

func (f parserFactoryFn) new(r messageReader) parser {
	return f(r)
}

type Peer struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	conn                  connection
	parserFactory         parserFactory
	outgoingTransactionCh chan keyedResponseHandler
	outgoingCtrlMessageCh chan message
	incomingCtrlMessageCh chan message
	subscriptionCh        chan *Subscription
	announcementCh        chan *Announcement
	trackLock             sync.RWMutex // TODO: Ensure lock is used for all tracks
	receiveTracks         map[uint64]*ReceiveTrack
	sendTracks            map[string]*SendTrack
	closeCh               chan struct{}
	closeOnce             sync.Once

	logger *log.Logger
}

func (p *Peer) CloseWithError(code uint64, reason string) error {
	return p.conn.CloseWithError(code, reason)
}

func newServerPeer(conn connection, pf parserFactory) (*Peer, error) {
	if conn == nil {
		return nil, errClosed
	}
	logger := log.New(os.Stdout, "MOQ_SERVER", log.LstdFlags)
	if pf == nil {
		pf = newLoggingParserFactory(logger)
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	p := &Peer{
		ctx:                   ctx,
		ctxCancel:             cancelFunc,
		conn:                  conn,
		parserFactory:         pf,
		outgoingTransactionCh: make(chan keyedResponseHandler),
		outgoingCtrlMessageCh: make(chan message),
		incomingCtrlMessageCh: make(chan message),
		subscriptionCh:        make(chan *Subscription, 10),
		announcementCh:        make(chan *Announcement, 10),
		trackLock:             sync.RWMutex{},
		receiveTracks:         map[uint64]*ReceiveTrack{},
		sendTracks:            map[string]*SendTrack{},
		closeCh:               make(chan struct{}),
		closeOnce:             sync.Once{},
		logger:                logger,
	}
	return p, p.runServerHandshake()
}

func newClientPeer(conn connection, role Role, pf parserFactory) (*Peer, error) {
	if conn == nil {
		return nil, errClosed
	}
	logger := log.New(os.Stdout, "MOQ_SERVER", log.LstdFlags)
	if pf == nil {
		pf = newLoggingParserFactory(logger)
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	p := &Peer{
		ctx:                   ctx,
		ctxCancel:             cancelFunc,
		conn:                  conn,
		parserFactory:         pf,
		outgoingTransactionCh: make(chan keyedResponseHandler),
		outgoingCtrlMessageCh: make(chan message),
		incomingCtrlMessageCh: make(chan message),
		subscriptionCh:        make(chan *Subscription, 10),
		announcementCh:        make(chan *Announcement, 10),
		trackLock:             sync.RWMutex{},
		receiveTracks:         map[uint64]*ReceiveTrack{},
		sendTracks:            map[string]*SendTrack{},
		closeCh:               make(chan struct{}),
		closeOnce:             sync.Once{},
		logger:                logger,
	}
	return p, p.runClientHandshake(role)
}

func (p *Peer) runServerHandshake() error {
	s, err := p.conn.AcceptStream(p.ctx)
	if err != nil {
		return err
	}
	pa := p.parserFactory.new(quicvarint.NewReader(s))
	m, err := pa.parse()
	if err != nil {
		return err
	}
	msg, ok := m.(*clientSetupMessage)
	if !ok {
		return errUnexpectedMessage
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, DRAFT_IETF_MOQ_TRANSPORT_01) {
		return errUnsupportedVersion
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		return errMissingRoleParameter
	}
	// TODO: save role parameter
	ssm := serverSetupMessage{
		SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
		SetupParameters: map[uint64]parameter{},
	}
	buf := ssm.append(make([]byte, 0, 1500))
	_, err = s.Write(buf)
	if err != nil {
		return err
	}
	go p.run(s, false)
	return nil
}

func (p *Peer) runClientHandshake(clientRole Role) error {
	s, err := p.conn.OpenStreamSync(p.ctx)
	if err != nil {
		return err
	}
	csm := clientSetupMessage{
		SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: uint64(clientRole),
			},
		},
	}
	buf := csm.append(make([]byte, 0, 1500))
	_, err = s.Write(buf)
	if err != nil {
		return err
	}
	msgParser := p.parserFactory.new(quicvarint.NewReader(s))
	msg, err := msgParser.parse()
	if err != nil {
		return err
	}
	ssm, ok := msg.(*serverSetupMessage)
	if !ok {
		return errUnexpectedMessage
	}
	if !slices.Contains(csm.SupportedVersions, ssm.SelectedVersion) {
		return errUnsupportedVersion
	}
	go p.run(s, false)
	return nil
}

func (p *Peer) run(s stream, enableDatagrams bool) {
	go p.controlLoop(s)
	go p.ctrlStreamReadLoop(s)
	go p.ctrlStreamWriteLoop(s)
	go p.acceptUnidirectionalStreams()
	if enableDatagrams {
		go p.acceptDatagrams()
	}
}

func (p *Peer) Close() error {
	p.closeOnce.Do(func() {
		close(p.closeCh)
	})
	<-p.ctx.Done()
	return nil
}

func (p *Peer) remoteClose(err error) {
	p.closeOnce.Do(func() {
		close(p.closeCh)
	})
}

func (p *Peer) ctrlStreamWriteLoop(s sendStream) {
	for {
		select {
		case <-p.ctx.Done():
			return
		case msg := <-p.outgoingCtrlMessageCh:
			buf := make([]byte, 0, 1500)
			buf = msg.append(buf)
			_, err := s.Write(buf)
			if err != nil {
				p.logger.Println(err)
				return
			}
		}
	}
}

func (p *Peer) ctrlStreamReadLoop(s receiveStream) {
	msgParser := p.parserFactory.new(quicvarint.NewReader(s))
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		msg, err := msgParser.parse()
		if err != nil {
			p.logger.Printf("got parsing error: %v", err)
			p.remoteClose(err)
			return
		}
		switch v := msg.(type) {
		case *subscribeRequestMessage, *subscribeOkMessage,
			*subscribeErrorMessage, *announceMessage, *announceOkMessage,
			*announceErrorMessage, *unannounceMessage, *unsubscribeMessage,
			*subscribeFinMessage, *subscribeRstMessage, *goAwayMessage,
			*clientSetupMessage, *serverSetupMessage:

			p.incomingCtrlMessageCh <- v
		default:
			p.logger.Printf("got unexpected message: %v", v)
			panic(errUnexpectedMessage)
		}
	}
}

func (p *Peer) readMessages(r messageReader) {
	msgParser := p.parserFactory.new(quicvarint.NewReader(r))
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		msg, err := msgParser.parse()
		if err != nil {
			panic(err)
		}
		switch v := msg.(type) {
		case *objectMessage:
			if err := p.handleObjectMessage(v); err != nil {
				panic(err)
			}
		default:
			p.logger.Printf("got unexpected message: %v", v)
			panic(errUnexpectedMessage)
		}
	}
}

type messageKey struct {
	mt messageType
	id string
}

type keyer interface {
	key() messageKey
}

type keyedMessage interface {
	message
	keyer
}

type responseHandler interface {
	handle(message)
}

type keyedResponseHandler interface {
	keyedMessage
	responseHandler
}

func (p *Peer) controlLoop(ctrlStream stream) {
	defer p.logger.Println("exit controlStreamLoop")
	defer p.ctxCancel()

	transactions := make(map[messageKey]keyedMessage)
	for {
		select {
		case <-p.closeCh:
			return
		case m := <-p.incomingCtrlMessageCh:
			switch v := m.(type) {
			case *subscribeRequestMessage:
				go func() {
					res := p.handleSubscribeRequest(v)
					p.logger.Printf("sending subscribe request response: %v", res)
					p.outgoingCtrlMessageCh <- res
				}()
			case *announceMessage:
				go func() {
					res := p.handleAnnounceMessage(v)
					p.logger.Printf("sending announce response: %v", res)
					p.outgoingCtrlMessageCh <- res
				}()
			case *goAwayMessage:
				p.handleGoAwayMessage(v)
			case keyedMessage:
				t, ok := transactions[v.key()]
				if !ok {
					// TODO: Error: This an error, because all keyed messages
					// that occur without responding to a transaction started by
					// us should be handled by the case above. I.e., if we get
					// an Ok or Error for Announce or subscribe, that should
					// only happen when we also stored an associated transaction
					// earlier.
					p.logger.Printf("unexpected unkeyed message")
					continue
				}
				rh, ok := t.(responseHandler)
				if !ok {
					p.logger.Printf("unexpected message without responseHandler")
					continue
				}
				rh.handle(v)
			default:
				p.logger.Printf("got unexpected message: %v", v)
				continue
			}
		case m := <-p.outgoingTransactionCh:
			transactions[m.key()] = m
			p.outgoingCtrlMessageCh <- m
		}
	}
}

func (p *Peer) acceptUnidirectionalStreams() {
	defer p.logger.Println("accept uni stream loop exit")
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		stream, err := p.conn.AcceptUniStream(context.TODO())
		if err != nil {
			p.logger.Print(err)
			return
		}
		p.logger.Println("GOT UNI STREAM")
		go p.readMessages(quicvarint.NewReader(stream))
	}
}

func (p *Peer) acceptDatagrams() {
	defer p.logger.Println("exit acceptDatagrams loop")
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		dgram, err := p.conn.ReceiveMessage(context.TODO())
		if err != nil {
			panic(err)
		}
		go p.readMessages(bytes.NewReader(dgram))
	}
}

func (p *Peer) handleObjectMessage(msg *objectMessage) error {
	t, ok := p.receiveTracks[msg.TrackID]
	if !ok {
		// handle unknown track?
		p.logger.Printf("got message for unknown track: %v", msg)
		return errUnknownTrack
	}
	return t.push(msg)
}

func (p *Peer) handleSubscribeRequest(msg *subscribeRequestMessage) message {
	s := &Subscription{
		lock:        sync.RWMutex{},
		track:       newSendTrack(p.conn),
		responseCh:  make(chan error),
		closeCh:     make(chan struct{}),
		expires:     0,
		namespace:   msg.TrackNamespace,
		trackname:   msg.TrackName,
		startGroup:  msg.StartGroup,
		startObject: msg.StartObject,
		endGroup:    msg.EndGroup,
		endObject:   msg.EndObject,
		parameters:  msg.Parameters,
	}
	select {
	case <-p.ctx.Done():
		return &subscribeErrorMessage{
			TrackNamespace: s.namespace,
			TrackName:      s.trackname,
			ErrorCode:      0, // TODO: set correct error code
			ReasonPhrase:   "peer closed",
		}
	case p.subscriptionCh <- s:
	}
	select {
	case <-p.ctx.Done():
		close(s.closeCh)
	case err := <-s.responseCh:
		if err != nil {
			return &subscribeErrorMessage{
				TrackNamespace: s.namespace,
				TrackName:      s.trackname,
				ErrorCode:      0, // TODO: Implement a custom error type including the code?
				ReasonPhrase:   fmt.Sprintf("subscription rejected: %v", err),
			}
		}
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	p.trackLock.Lock()
	defer p.trackLock.Unlock()
	p.sendTracks[msg.key().id] = s.track
	return &subscribeOkMessage{
		TrackNamespace: s.namespace,
		TrackName:      s.trackname,
		TrackID:        s.TrackID(),
		Expires:        s.expires,
	}
}

func (p *Peer) handleAnnounceMessage(msg *announceMessage) message {
	a := &Announcement{
		responseCh: make(chan error),
		closeCh:    make(chan struct{}),
		namespace:  msg.TrackNamespace,
		parameters: msg.TrackRequestParameters,
	}
	select {
	case <-p.ctx.Done():
		return &announceErrorMessage{
			TrackNamespace: msg.TrackNamespace,
			ErrorCode:      0, // TODO: Set correct code?
			ReasonPhrase:   "peer closed",
		}
	case p.announcementCh <- a:
	}
	select {
	case <-p.ctx.Done():
		close(a.closeCh)
	case err := <-a.responseCh:
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

func (p *Peer) handleGoAwayMessage(msg *goAwayMessage) {
	panic("TODO")
}

type ctrlMessage struct {
	keyedMessage
	responseCh chan message
}

func (m *ctrlMessage) handle(msg message) {
	m.responseCh <- msg
}

func (p *Peer) ReadSubscription(ctx context.Context) (*Subscription, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.ctx.Done():
		return nil, errors.New("peer closed") // TODO: Better error message including a reason?
	case s := <-p.subscriptionCh:
		return s, nil
	}
}

func (p *Peer) ReadAnnouncement(ctx context.Context) (*Announcement, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.ctx.Done():
		return nil, errors.New("peer closed") // TODO: Better error message including a reason?
	case a := <-p.announcementCh:
		return a, nil
	}
}

func (p *Peer) Announce(namespace string) error {
	if len(namespace) == 0 {
		return errInvalidTrackNamespace
	}
	am := &announceMessage{
		TrackNamespace:         namespace,
		TrackRequestParameters: map[uint64]parameter{},
	}
	responseCh := make(chan message)
	p.logger.Printf("sending announce: %v", am)
	select {
	case p.outgoingTransactionCh <- &ctrlMessage{
		keyedMessage: am,
		responseCh:   responseCh,
	}:
	case <-p.ctx.Done():
		return errClosed
	}
	var resp message
	select {
	case resp = <-responseCh:
	case <-time.After(time.Second): // TODO: Make timeout configurable?
		panic("TODO: timeout error")
	case <-p.ctx.Done():
		return errClosed
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		if v.TrackNamespace != am.TrackNamespace {
			panic("TODO")
		}
	case *announceErrorMessage:
		return errors.New(v.ReasonPhrase) // TODO: Wrap error string?
	default:
		return errUnexpectedMessage
	}
	return nil
}

func (p *Peer) Subscribe(namespace, trackname, auth string) (*ReceiveTrack, error) {
	sm := &subscribeRequestMessage{
		TrackNamespace: namespace,
		TrackName:      trackname,
		StartGroup:     location{},
		StartObject:    location{},
		EndGroup:       location{},
		EndObject:      location{},
		Parameters:     parameters{},
	}
	if len(auth) > 0 {
		sm.Parameters[authorizationParameterKey] = stringParameter{
			k: authorizationParameterKey,
			v: auth,
		}
	}
	responseCh := make(chan message)
	p.logger.Printf("sending subscribeRequest: %v", sm)
	select {
	case p.outgoingTransactionCh <- &ctrlMessage{
		keyedMessage: sm,
		responseCh:   responseCh,
	}:
	case <-p.ctx.Done():
		return nil, errClosed
	case <-time.After(time.Second):
		panic("TODO: timeout error")
	}
	var resp message
	select {
	case resp = <-responseCh:
	case <-time.After(time.Second): // TODO: Make timeout configurable?
		panic("TODO: timeout error")
	case <-p.ctx.Done():
		return nil, errClosed
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		if v.key().id != sm.key().id {
			panic("TODO")
		}
		t := newReceiveTrack()
		p.receiveTracks[v.TrackID] = t
		return t, nil

	case *subscribeErrorMessage:
		return nil, errors.New(v.ReasonPhrase)
	}
	return nil, errUnexpectedMessage
}
