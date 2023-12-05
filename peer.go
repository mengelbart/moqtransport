package moqtransport

import (
	"bytes"
	"context"
	"errors"
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

type SubscriptionHandler func(namespace, trackname string, track *SendTrack) (uint64, time.Duration, error)

type AnnouncementHandler func(string) error

type Peer struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	conn                  connection
	outgoingCtrlMessageCh chan message
	incomingCtrlMessageCh chan message
	receiveTracks         map[uint64]*ReceiveTrack
	sendTracks            map[string]*SendTrack
	subscribeHandler      SubscriptionHandler
	announcementHandler   AnnouncementHandler
	closeCh               chan struct{}
	closeOnce             sync.Once
	newParser             parserFactory

	logger *log.Logger
}

func (p *Peer) CloseWithError(code uint64, reason string) error {
	return p.conn.CloseWithError(code, reason)
}

type parser interface {
	parse() (message, error)
}

type parserFactory func(messageReader, *log.Logger) parser

func newServerPeer(conn connection, newParser parserFactory) (*Peer, error) {
	if conn == nil {
		return nil, errClosed
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := conn.AcceptStream(sctx)
	if err != nil {
		return nil, err
	}
	if newParser == nil {
		newParser = func(mr messageReader, l *log.Logger) parser {
			return &loggingParser{
				logger: l,
				reader: mr,
			}
		}
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	p := &Peer{
		ctx:                   ctx,
		ctxCancel:             cancelFunc,
		conn:                  conn,
		outgoingCtrlMessageCh: make(chan message),
		incomingCtrlMessageCh: make(chan message),
		receiveTracks:         map[uint64]*ReceiveTrack{},
		sendTracks:            map[string]*SendTrack{},
		subscribeHandler:      nil,
		announcementHandler:   nil,
		closeCh:               make(chan struct{}),
		closeOnce:             sync.Once{},
		newParser:             newParser,
		logger:                log.New(os.Stdout, "MOQ_SERVER: ", log.LstdFlags),
	}
	m, err := p.newParser(quicvarint.NewReader(s), p.logger).parse()
	if err != nil {
		return nil, err
	}
	msg, ok := m.(*clientSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, DRAFT_IETF_MOQ_TRANSPORT_01) {
		return nil, errUnsupportedVersion
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		return nil, errMissingRoleParameter
	}
	// TODO: save role parameter
	ssm := serverSetupMessage{
		SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
		SetupParameters: map[uint64]parameter{},
	}
	buf := ssm.append(make([]byte, 0, 1500))
	_, err = s.Write(buf)
	if err != nil {
		return nil, err
	}
	go p.controlStreamLoop(s)
	return p, nil
}

func newClientPeer(conn connection, newParser parserFactory, clientRole uint64) (*Peer, error) {
	if conn == nil {
		return nil, errClosed
	}
	if newParser == nil {
		newParser = func(mr messageReader, l *log.Logger) parser {
			return &loggingParser{
				logger: l,
				reader: mr,
			}
		}
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := conn.OpenStreamSync(sctx)
	if err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	p := &Peer{
		ctx:                   ctx,
		ctxCancel:             cancelFunc,
		conn:                  conn,
		outgoingCtrlMessageCh: make(chan message),
		incomingCtrlMessageCh: make(chan message),
		receiveTracks:         map[uint64]*ReceiveTrack{},
		sendTracks:            map[string]*SendTrack{},
		subscribeHandler:      nil,
		announcementHandler:   nil,
		closeCh:               make(chan struct{}),
		closeOnce:             sync.Once{},
		newParser:             newParser,
		logger:                log.New(os.Stdout, "MOQ_CLIENT: ", log.LstdFlags),
	}
	csm := clientSetupMessage{
		SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: clientRole,
			},
		},
	}
	buf := csm.append(make([]byte, 0, 1500))
	_, err = s.Write(buf)
	if err != nil {
		return nil, err
	}
	msg, err := p.newParser(quicvarint.NewReader(s), p.logger).parse()
	if err != nil {
		return nil, err
	}
	ssm, ok := msg.(*serverSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	if !slices.Contains(csm.SupportedVersions, ssm.SelectedVersion) {
		return nil, errUnsupportedVersion
	}
	go p.controlStreamLoop(s)
	return p, nil
}

func (p *Peer) Run(enableDatagrams bool) {
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

func (p *Peer) readControlMessages(r messageReader) {
	p.logger.Println("readControlMessages")
	defer p.logger.Println("exit readControlMessages")
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		msg, err := p.newParser(r, p.logger).parse()
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
			panic(errUnexpectedMessage)
		}
	}
}

func (p *Peer) readMessages(r messageReader) {
	p.logger.Println("readControlMessages")
	defer p.logger.Println("exit readMessages")
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		msg, err := p.newParser(r, p.logger).parse()
		if err != nil {
			panic(err)
		}
		switch v := msg.(type) {
		case *objectMessage:
			if err := p.handleObjectMessage(v); err != nil {
				panic(err)
			}
		default:
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

func (p *Peer) controlStreamLoop(ctrlStream stream) {
	defer p.logger.Println("exit controlStreamLoop")
	defer p.ctxCancel()
	defer ctrlStream.Close()

	go p.readControlMessages(quicvarint.NewReader(ctrlStream))

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
					return
				}
				rh, ok := t.(responseHandler)
				if !ok {
					p.logger.Printf("unexpected message without responseHandler")
				}
				rh.handle(v)
			default:
				return
			}
		case m := <-p.outgoingCtrlMessageCh:
			if krh, ok := m.(keyedResponseHandler); ok {
				transactions[krh.key()] = krh
			}
			buf := make([]byte, 0, 1500)
			buf = m.append(buf)
			_, err := ctrlStream.Write(buf)
			if err != nil {
				p.logger.Println(err)
				return
			}
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
	if p.subscribeHandler == nil {
		panic("TODO")
	}
	t := newSendTrack(p.conn)
	p.sendTracks[msg.key().id] = t
	id, expires, err := p.subscribeHandler(msg.TrackNamespace, msg.TrackName, t)
	if err != nil {
		return &subscribeErrorMessage{
			TrackNamespace: msg.TrackNamespace,
			TrackName:      msg.TrackName,
			ErrorCode:      GenericErrorCode,
			ReasonPhrase:   "failed to handle subscription",
		}
	}
	t.id = id
	return &subscribeOkMessage{
		TrackNamespace: msg.TrackNamespace,
		TrackName:      msg.TrackName,
		TrackID:        id,
		Expires:        expires,
	}
}

func (p *Peer) handleAnnounceMessage(msg *announceMessage) message {
	if p.announcementHandler == nil {
		panic("TODO")
	}
	if err := p.announcementHandler(msg.TrackNamespace); err != nil {
		return &announceErrorMessage{
			TrackNamespace: msg.TrackNamespace,
			ErrorCode:      0,
			ReasonPhrase:   "failed to handle announcement",
		}
	}
	return &announceOkMessage{
		TrackNamespace: msg.TrackNamespace,
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
	case p.outgoingCtrlMessageCh <- &ctrlMessage{
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
		Parameters: parameters{
			authorizationParameterKey: stringParameter{
				k: authorizationParameterKey,
				v: auth,
			},
		},
	}
	responseCh := make(chan message)
	p.logger.Printf("sending subscribeRequest: %v", sm)
	select {
	case p.outgoingCtrlMessageCh <- &ctrlMessage{
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

func (p *Peer) OnAnnouncement(callback AnnouncementHandler) {
	p.announcementHandler = callback
}

func (p *Peer) OnSubscription(callback SubscriptionHandler) {
	p.subscribeHandler = callback
}
