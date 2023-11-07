package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"time"

	"github.com/quic-go/quic-go/quicvarint"
	"golang.org/x/exp/slices"
)

var (
	errUnexpectedMessage     = errors.New("got unexpected message")
	errInvalidTrackNamespace = errors.New("got invalid tracknamespace")
	errClosed                = errors.New("connection was closed")
	errUnsupportedVersion    = errors.New("unsupported version")
	errMissingRoleParameter  = errors.New("missing role parameter")
	errGoAway                = errors.New("received go away from peer")
)

// TODO: Streams must be wrapped properly for quic and webtransport The
// interfaces need to add CancelRead and CancelWrite for STOP_SENDING and
// RESET_STREAM purposes. The interface should allow implementations for quic
// and webtransport.
type stream interface {
	readStream
	sendStream
}

type readStream interface {
	io.Reader
}

type sendStream interface {
	io.WriteCloser
}

type connection interface {
	OpenStream() (stream, error)
	OpenStreamSync(context.Context) (stream, error)
	OpenUniStream() (sendStream, error)
	OpenUniStreamSync(context.Context) (sendStream, error)
	AcceptStream(context.Context) (stream, error)
	AcceptUniStream(context.Context) (readStream, error)
	ReceiveMessage(context.Context) ([]byte, error)
	CloseWithError(uint64, string) error
}

type SubscriptionHandler func(namespace, trackname string, track *SendTrack) (uint64, time.Duration, error)

type AnnouncementHandler func(string) error

type Peer struct {
	conn                connection
	inMsgCh             chan message
	ctrlMessageCh       chan message
	ctrlStream          stream
	role                role
	receiveTracks       map[uint64]*ReceiveTrack
	sendTracks          map[string]*SendTrack
	subscribeHandler    SubscriptionHandler
	announcementHandler AnnouncementHandler
	closeCh             chan struct{}
}

func (p *Peer) CloseWithError(code uint64, reason string) error {
	return p.conn.CloseWithError(code, reason)
}

func newServerPeer(ctx context.Context, conn connection) (*Peer, error) {
	s, err := conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	m, err := readNext(quicvarint.NewReader(s))
	if err != nil {
		return nil, err
	}
	msg, ok := m.(*clientSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.supportedVersions, DRAFT_IETF_MOQ_TRANSPORT_01) {
		return nil, errUnsupportedVersion
	}
	_, ok = msg.setupParameters[roleParameterKey]
	if !ok {
		return nil, errMissingRoleParameter
	}
	// TODO: save role parameter
	ssm := serverSetupMessage{
		selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
		setupParameters: map[uint64]parameter{},
	}
	buf := ssm.append(make([]byte, 0, 1500))
	_, err = s.Write(buf)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		conn:                conn,
		inMsgCh:             make(chan message),
		ctrlMessageCh:       make(chan message),
		ctrlStream:          s,
		role:                serverRole,
		receiveTracks:       map[uint64]*ReceiveTrack{},
		sendTracks:          map[string]*SendTrack{},
		subscribeHandler:    nil,
		announcementHandler: nil,
		closeCh:             make(chan struct{}),
	}
	return p, nil
}

func newClientPeer(ctx context.Context, conn connection) (*Peer, error) {
	s, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	csm := clientSetupMessage{
		supportedVersions: []version{version(DRAFT_IETF_MOQ_TRANSPORT_01)},
		setupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: ingestionDeliveryRole,
			},
		},
	}
	buf := csm.append(make([]byte, 0, 1500))
	_, err = s.Write(buf)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		conn:                conn,
		inMsgCh:             make(chan message),
		ctrlMessageCh:       make(chan message),
		ctrlStream:          s,
		role:                clientRole,
		receiveTracks:       map[uint64]*ReceiveTrack{},
		sendTracks:          map[string]*SendTrack{},
		subscribeHandler:    nil,
		announcementHandler: nil,
		closeCh:             make(chan struct{}),
	}
	m, err := readNext(quicvarint.NewReader(s))
	if err != nil {
		return nil, err
	}
	ssm, ok := m.(*serverSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	if !slices.Contains(csm.supportedVersions, ssm.selectedVersion) {
		return nil, errUnsupportedVersion
	}
	return p, nil
}

func (p *Peer) Run(ctx context.Context, enableDatagrams bool) error {
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fs := []func(context.Context) error{
		p.controlStreamLoop,
		p.acceptUnidirectionalStreams,
		p.acceptBidirectionalStreams,
	}
	if enableDatagrams {
		fs = append(fs, p.acceptDatagrams)
	}

	for _, f := range fs {
		go func(ctx context.Context, f func(context.Context) error, ch chan<- error) {
			if err := f(ctx); err != nil {
				ch <- err
			}
		}(ctx, f, errCh)
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (p *Peer) readMessages(r messageReader, stream io.Reader) error {
	for {
		msg, err := readNext(r)
		if err != nil {
			return err
		}
		object, ok := msg.(*objectMessage)
		if !ok {
			return errUnexpectedMessage
		}
		p.handleObjectMessage(object)
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

func (p *Peer) controlStreamLoop(ctx context.Context) error {
	inCh := make(chan message)
	errCh := make(chan error)
	transactions := make(map[messageKey]keyedMessage)

	go func(s stream, ch chan<- message, errCh chan<- error) {
		for {
			msg, err := readNext(quicvarint.NewReader(s))
			if err != nil {
				errCh <- err
				return
			}
			ch <- msg
		}
	}(p.ctrlStream, inCh, errCh)
	for {
		select {
		case m := <-inCh:
			switch v := m.(type) {
			case *subscribeRequestMessage:
				go func() {
					p.ctrlMessageCh <- p.handleSubscribeRequest(v)
				}()
			case *announceMessage:
				go func() {
					p.ctrlMessageCh <- p.handleAnnounceMessage(v)
				}()
			case *goAwayMessage:
				return errGoAway
			case keyedMessage:
				t, ok := transactions[v.key()]
				if !ok {
					// TODO: Error: This an error, because all keyed messages
					// that occur without responding to a transaction started by
					// us should be handled by the case above. I.e., if we get
					// an Ok or Error for Announce or subscribe, that should
					// only happen when we also stored an associated transaction
					// earlier.
					return errors.New("unexpected unkeyed message")
				}
				rh, ok := t.(responseHandler)
				if !ok {
					return errors.New("unexpected message without responseHandler")
				}
				rh.handle(v)
			default:
				return errUnexpectedMessage
			}
		case m := <-p.ctrlMessageCh:
			if krh, ok := m.(keyedResponseHandler); ok {
				transactions[krh.key()] = krh
			}
			buf := make([]byte, 0, 1500)
			buf = m.append(buf)
			_, err := p.ctrlStream.Write(buf)
			if err != nil {
				return err
			}
		case err := <-errCh:
			return err
		}
	}
}

func (p *Peer) acceptBidirectionalStreams(ctx context.Context) error {
	for {
		s, err := p.conn.AcceptStream(ctx)
		if err != nil {
			return err
		}
		log.Println("accepted bidi stream")
		go func() {
			if err := p.readMessages(quicvarint.NewReader(s), s); err != nil {
				panic(err)
			}
		}()
	}
}

func (p *Peer) acceptUnidirectionalStreams(ctx context.Context) error {
	for {
		stream, err := p.conn.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		log.Println("accepted uni stream")
		go func() {
			if err := p.readMessages(quicvarint.NewReader(stream), stream); err != nil {
				panic(err)
			}
		}()
	}
}

func (p *Peer) acceptDatagrams(ctx context.Context) error {
	for {
		dgram, err := p.conn.ReceiveMessage(ctx)
		if err != nil {
			return err
		}
		r := bytes.NewReader(dgram)
		go func() {
			if err := p.readMessages(r, nil); err != nil {
				panic(err)
			}
		}()
	}
}

func (p *Peer) handleObjectMessage(msg *objectMessage) error {
	t, ok := p.receiveTracks[msg.trackID]
	if !ok {
		// handle unknown track?
		panic("TODO")
	}
	t.push(msg)
	return nil
}

func (p *Peer) handleSubscribeRequest(msg *subscribeRequestMessage) message {
	log.Printf("handling subscribe: namespace='%v', name='%v'\n", msg.trackNamespace, msg.trackName)
	if p.subscribeHandler == nil {
		panic("TODO")
	}
	t := newSendTrack(p.conn)
	p.sendTracks[msg.key().id] = t
	id, expires, err := p.subscribeHandler(msg.trackNamespace, msg.trackName, t)
	if err != nil {
		log.Println(err)
		return &subscribeErrorMessage{
			trackNamespace: msg.trackNamespace,
			trackName:      msg.trackName,
			errorCode:      GenericErrorCode,
			reasonPhrase:   "failed to handle subscription",
		}
	}
	t.id = id
	return &subscribeOkMessage{
		trackNamespace: msg.trackNamespace,
		trackName:      msg.trackName,
		trackID:        id,
		expires:        expires,
	}
}

func (p *Peer) handleAnnounceMessage(msg *announceMessage) message {
	log.Printf("handling announce: %v\n", msg.trackNamespace)
	if p.announcementHandler == nil {
		panic("TODO")
	}
	if err := p.announcementHandler(msg.trackNamespace); err != nil {
		return &announceErrorMessage{
			trackNamespace: msg.trackNamespace,
			errorCode:      0,
			reasonPhrase:   "failed to handle announcement",
		}
	}
	return &announceOkMessage{
		trackNamespace: msg.trackNamespace,
	}
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
		trackNamespace:         namespace,
		trackRequestParameters: map[uint64]parameter{},
	}
	responseCh := make(chan message)
	select {
	case p.ctrlMessageCh <- &ctrlMessage{
		keyedMessage: am,
		responseCh:   responseCh,
	}:
	case <-p.closeCh:
		return errClosed
	}
	var resp message
	select {
	case resp = <-responseCh:
	case <-time.After(time.Second): // TODO: Make timeout configurable?
		panic("TODO: timeout error")
	case <-p.closeCh:
		return errClosed
	}
	switch v := resp.(type) {
	case *announceOkMessage:
		if v.trackNamespace != am.trackNamespace {
			panic("TODO")
		}
	case *announceErrorMessage:
		return errors.New(v.reasonPhrase) // TODO: Wrap error string?
	default:
		return errUnexpectedMessage
	}
	return nil
}

func (p *Peer) Subscribe(namespace, trackname string) (*ReceiveTrack, error) {
	sm := &subscribeRequestMessage{
		trackNamespace: namespace,
		trackName:      trackname,
		startGroup:     location{},
		startObject:    location{},
		endGroup:       location{},
		endObject:      location{},
		parameters:     parameters{},
	}
	responseCh := make(chan message)
	select {
	case p.ctrlMessageCh <- &ctrlMessage{
		keyedMessage: sm,
		responseCh:   responseCh,
	}:
	case <-p.closeCh:
		return nil, errClosed
	case <-time.After(time.Second):
		panic("TODO: timeout error")
	}
	var resp message
	select {
	case resp = <-responseCh:
	case <-time.After(time.Second): // TODO: Make timeout configurable?
		panic("TODO: timeout error")
	case <-p.closeCh:
		return nil, errClosed
	}
	switch v := resp.(type) {
	case *subscribeOkMessage:
		if v.key().id != sm.key().id {
			panic("TODO")
		}
		t := newReceiveTrack()
		p.receiveTracks[v.trackID] = t
		return t, nil

	case *subscribeErrorMessage:
		return nil, errors.New(v.reasonPhrase)
	}
	return nil, errUnexpectedMessage
}

func (p *Peer) OnAnnouncement(callback AnnouncementHandler) {
	p.announcementHandler = callback
}

func (p *Peer) OnSubscription(callback SubscriptionHandler) {
	p.subscribeHandler = callback
}
