package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"

	"gitlab.lrz.de/cm/moqtransport/varint"
	"golang.org/x/exp/slices"
)

var (
	errUnexpectedMessage = errors.New("got unexpected message")
)

type Stream interface {
	ReadStream
	SendStream
}

type ReadStream interface {
	io.Reader
}

type SendStream interface {
	io.ReadWriteCloser
}

type Connection interface {
	OpenStreamSync(context.Context) (Stream, error)
	AcceptStream(context.Context) (Stream, error)
	AcceptUniStream(context.Context) (ReadStream, error)
	ReceiveMessage(context.Context) ([]byte, error)
}

type msgContext struct {
	msg            Message
	stream         io.Reader
	responseWriter io.WriteCloser
}

type Peer struct {
	conn          Connection
	inMsgCh       chan *msgContext
	outMsgCh      chan *msgContext
	role          role
	receiveTracks map[uint64]*ReceiveTrack
}

func NewServerPeer(ctx context.Context, conn Connection) (*Peer, error) {
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	m, err := ReadNext(varint.NewReader(stream), serverRole)
	if err != nil {
		return nil, err
	}
	msg, ok := m.(*ClientSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}
	// TODO: Algorithm to select best matching version
	if !slices.Contains(msg.SupportedVersions, DRAFT_IETF_MOQ_TRANSPORT_00) {
		// TODO: Close conn with error
		log.Println("TODO: Close conn with error")
		return nil, nil
	}
	_, ok = msg.SetupParameters[roleParameterKey]
	if !ok {
		// ERROR: role is required
		log.Println("TODO: ERROR: role is required")
		return nil, nil
	}
	// TODO: save role parameter
	ssm := ServerSetupMessage{
		SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
		SetupParameters: map[parameterKey]Parameter{},
	}
	buf := ssm.Append(make([]byte, 0, 1500))
	_, err = stream.Write(buf)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		conn:     conn,
		inMsgCh:  make(chan *msgContext),
		outMsgCh: make(chan *msgContext),
		role:     serverRole,
	}
	go p.controlStreamLoop(ctx, stream)
	go p.handle(ctx)
	return p, nil
}

func NewClientPeer(ctx context.Context, conn Connection) (*Peer, error) {
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	csm := ClientSetupMessage{
		SupportedVersions: []Version{Version(DRAFT_IETF_MOQ_TRANSPORT_00)},
		SetupParameters: map[parameterKey]Parameter{
			roleParameterKey: ingestionDeliveryRole,
		},
	}
	buf := csm.Append(make([]byte, 0, 1500))
	_, err = stream.Write(buf)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		conn:     conn,
		inMsgCh:  make(chan *msgContext),
		outMsgCh: make(chan *msgContext),
		role:     clientRole,
	}
	m, err := ReadNext(varint.NewReader(stream), clientRole)
	if err != nil {
		return nil, err
	}
	_, ok := m.(*ServerSetupMessage)
	if !ok {
		return nil, errUnexpectedMessage
	}

	go p.handle(ctx)
	go p.readMessages(varint.NewReader(stream), stream)
	return p, nil
}

func (p *Peer) readMessages(r MessageReader, stream io.Reader) {
	var rw io.WriteCloser
	if s, ok := stream.(io.WriteCloser); ok {
		rw = s
	}
	for {
		log.Println("reading message")
		msg, err := ReadNext(r, p.role)
		if err != nil {
			// TODO: Handle/log error?
			log.Println(err)
			return
		}
		log.Printf("read message: %v\n", msg)
		p.inMsgCh <- &msgContext{
			msg:            msg,
			stream:         stream,
			responseWriter: rw,
		}
	}
}

func (p *Peer) controlStreamLoop(ctx context.Context, s Stream) {
	inCh := make(chan *msgContext)
	errCh := make(chan error)
	go func(s Stream, ch chan<- *msgContext, errCh chan<- error) {
		for {
			msg, err := ReadNext(varint.NewReader(s), p.role)
			if err != nil {
				errCh <- err
				return
			}
			ch <- &msgContext{
				msg:            msg,
				stream:         s,
				responseWriter: nil,
			}
		}
	}(s, inCh, errCh)
	for {
		select {
		case m := <-inCh:
			p.inMsgCh <- m
		case m := <-p.outMsgCh:
			buf := make([]byte, 0, 1500)
			buf = m.msg.Append(buf)
			_, err := s.Write(buf)
			if err != nil {
				// TODO
				log.Println(err)
			}
		case err := <-errCh:
			// TODO
			log.Println(err)
		}
	}
}

func (p *Peer) acceptBidirectionalStreams(ctx context.Context) {
	for {
		stream, err := p.conn.AcceptStream(ctx)
		if err != nil {
			// TODO: Handle/log error?
			log.Println(err)
			return
		}
		log.Println("got new bidirectional stream")
		go p.readMessages(varint.NewReader(stream), stream)
	}
}

func (p *Peer) acceptUnidirectionalStreams(ctx context.Context) {
	for {
		stream, err := p.conn.AcceptUniStream(ctx)
		if err != nil {
			// TODO: Handle/log error?
			log.Println(err)
			return
		}
		log.Println("got new unidirectional stream")
		go p.readMessages(varint.NewReader(stream), stream)
	}
}

func (p *Peer) acceptDatagrams(ctx context.Context) {
	for {
		dgram, err := p.conn.ReceiveMessage(ctx)
		if err != nil {
			// TODO: Handle/log error?
			log.Println(err)
			return
		}
		r := bytes.NewReader(dgram)
		go p.readMessages(r, nil)
	}
}

func (p *Peer) handle(ctx context.Context) error {
	errCh := make(chan error)

	go p.acceptBidirectionalStreams(ctx)
	go p.acceptUnidirectionalStreams(ctx)
	// TODO: Configure if datagrams enabled?
	go p.acceptDatagrams(ctx)

	for {
		select {
		case err := <-errCh:
			// TODO: Handle errors
			log.Println(err)

		case m := <-p.outMsgCh:
			log.Printf("TODO: send message: %v\n", m.msg)

		case m := <-p.inMsgCh:
			log.Printf("received message: %v\n", m.msg)
			switch v := m.msg.(type) {
			case *ObjectMessage:
				p.handleObjectMessage(v)

			case *ClientSetupMessage:
				// ERROR errUnexpectedMessage?

			case *ServerSetupMessage:
				// ERROR errUnexpectedMessage?

			case *SubscribeRequestMessage:
				p.handleSubscribeRequest()

			case *SubscribeOkMessage:
				// ERROR errUnexpectedMessage?

			case *SubscribeErrorMessage:
				// ERROR errUnexpectedMessage?

			case *AnnounceMessage:
				p.handleAnnounceMessage()

			case *AnnounceOkMessage:
				// ERROR errUnexpectedMessage?

			case *AnnounceErrorMessage:
				// ERROR errUnexpectedMessage?

			case *GoAwayMessage:
			default:
				// ERROR errUnexpectedMessage?
				return errUnexpectedMessage
			}
		}
	}
}

func (p *Peer) Announce() (*SendTrack, error) {
	return nil, nil
}

func (p *Peer) Subscribe() (*ReceiveTrack, error) {
	return nil, nil
}

func (p *Peer) OnAnnouncement(callback func()) {
}

func (p *Peer) handleObjectMessage(msg *ObjectMessage) error {
	t, ok := p.receiveTracks[msg.TrackID]
	if !ok {
		// handle unknown track?
	}
	t.push(msg)
	return nil
}

func (p *Peer) handleSubscribeRequest() error {
	return nil
}

func (p *Peer) handleAnnounceMessage() error {
	return nil
}
