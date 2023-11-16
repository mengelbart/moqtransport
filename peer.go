package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/quic-go/quic-go"
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
	receiveTracks       map[uint64]*ReceiveTrack
	sendTracks          map[string]*SendTrack
	subscribeHandler    SubscriptionHandler
	announcementHandler AnnouncementHandler
	closeCh             chan struct{}

	logger *log.Logger
}

func (p *Peer) CloseWithError(code uint64, reason string) error {
	return p.conn.CloseWithError(code, reason)
}

func newServerPeer(ctx context.Context, conn connection) (*Peer, error) {
	s, err := conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		conn:                conn,
		inMsgCh:             make(chan message),
		ctrlMessageCh:       make(chan message),
		ctrlStream:          s,
		receiveTracks:       map[uint64]*ReceiveTrack{},
		sendTracks:          map[string]*SendTrack{},
		subscribeHandler:    nil,
		announcementHandler: nil,
		closeCh:             make(chan struct{}),
		logger:              log.New(os.Stdout, "MOQ_SERVER: ", log.LstdFlags),
	}
	m, err := p.parseMessage(quicvarint.NewReader(s))
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
	return p, nil
}

func newClientPeer(ctx context.Context, conn connection, clientRole uint64) (*Peer, error) {
	s, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		conn:                conn,
		inMsgCh:             make(chan message),
		ctrlMessageCh:       make(chan message),
		ctrlStream:          s,
		receiveTracks:       map[uint64]*ReceiveTrack{},
		sendTracks:          map[string]*SendTrack{},
		subscribeHandler:    nil,
		announcementHandler: nil,
		closeCh:             make(chan struct{}),
		logger:              log.New(os.Stdout, "MOQ_CLIENT: ", log.LstdFlags),
	}
	csm := clientSetupMessage{
		SupportedVersions: []version{version(DRAFT_IETF_MOQ_TRANSPORT_01)},
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
	m, err := p.parseMessage(quicvarint.NewReader(s))
	if err != nil {
		return nil, err
	}
	ssm, ok := m.(*serverSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	if !slices.Contains(csm.SupportedVersions, ssm.SelectedVersion) {
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

func (p *Peer) parseMessage(r messageReader) (message, error) {
	ps := &parser{
		logger: p.logger,
		reader: r,
	}
	msg, err := ps.readNext()
	if err != nil {
		p.logger.Printf("failed to parse message: %v", err)
		return nil, err
	}
	p.logger.Printf("parsed message: %v", msg)
	return msg, nil
}

func (p *Peer) readMessages(r messageReader, stream io.Reader) error {
	for {
		msg, err := p.parseMessage(r)
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
		defer s.Close()
		for {
			msg, err := p.parseMessage(quicvarint.NewReader(s))
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

func (p *Peer) acceptUnidirectionalStreams(ctx context.Context) error {
	for {
		stream, err := p.conn.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		go func() {
			err := p.readMessages(quicvarint.NewReader(stream), stream)
			if err != nil {
				log.Printf("read message from uni stream err: %v", err)
				if streamErr, ok := err.(*quic.StreamError); ok {
					log.Printf("stream err: %v", streamErr)
				}
				return
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
	t, ok := p.receiveTracks[msg.TrackID]
	if !ok {
		// handle unknown track?
		p.logger.Printf("got message for unknown track: %v", msg)
		return nil
	}
	t.push(msg)
	return nil
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
