package xnetquic

import (
	"context"
	"errors"
	"sync"

	"github.com/mengelbart/moqtransport"
	"golang.org/x/net/quic"
)

type connection struct {
	ctx         context.Context
	cancelCtx   context.CancelFunc
	conn        *quic.Conn
	lock        sync.Mutex
	perspective moqtransport.Perspective
	bidiStreams chan *quic.Stream
	uniStreams  chan *quic.Stream
}

func NewServer(conn *quic.Conn) moqtransport.Connection {
	return New(conn, moqtransport.PerspectiveServer)
}

func NewClient(conn *quic.Conn) moqtransport.Connection {
	return New(conn, moqtransport.PerspectiveClient)
}

func New(conn *quic.Conn, perspective moqtransport.Perspective) moqtransport.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	c := &connection{
		ctx:         ctx,
		cancelCtx:   cancel,
		conn:        conn,
		lock:        sync.Mutex{},
		bidiStreams: make(chan *quic.Stream, 100),
		uniStreams:  make(chan *quic.Stream, 100),
	}
	go c.accept()
	return c
}

func (c *connection) accept() {
	for {
		s, err := c.conn.AcceptStream(c.ctx)
		if err != nil {
			return
		}
		if s.IsReadOnly() {
			select {
			case c.uniStreams <- s:
			case <-c.ctx.Done():
				return
			}
		} else {
			select {
			case c.bidiStreams <- s:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

// AcceptStream implements moqtransport.Connection.
func (c *connection) AcceptStream(ctx context.Context) (moqtransport.Stream, error) {
	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	case s := <-c.bidiStreams:
		return &Stream{
			stream: s,
		}, nil
	}
}

// AcceptUniStream implements moqtransport.Connection.
func (c *connection) AcceptUniStream(ctx context.Context) (moqtransport.ReceiveStream, error) {
	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	case s := <-c.uniStreams:
		return &Stream{
			stream: s,
		}, nil
	}
}

// OpenStream implements moqtransport.Connection.
func (c *connection) OpenStream() (moqtransport.Stream, error) {
	return c.OpenStreamSync(context.Background())
}

// OpenStreamSync implements moqtransport.Connection.
func (c *connection) OpenStreamSync(ctx context.Context) (moqtransport.Stream, error) {
	s, err := c.conn.NewStream(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

// OpenUniStream implements moqtransport.Connection.
func (c *connection) OpenUniStream() (moqtransport.SendStream, error) {
	return c.OpenUniStreamSync(context.TODO())
}

// OpenUniStreamSync implements moqtransport.Connection.
func (c *connection) OpenUniStreamSync(ctx context.Context) (moqtransport.SendStream, error) {
	s, err := c.conn.NewSendOnlyStream(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

// ReceiveDatagram implements moqtransport.Connection.
func (*connection) ReceiveDatagram(context.Context) ([]byte, error) {
	return nil, errors.New("x/net/quic implementation does not support datagram")
}

// SendDatagram implements moqtransport.Connection.
func (*connection) SendDatagram([]byte) error {
	return errors.New("x/net/quic implementation does not support datagram")
}

// CloseWithError implements moqtransport.Connection.
func (c *connection) CloseWithError(code uint64, reason string) error {
	c.conn.Abort(&quic.ApplicationError{
		Code:   code,
		Reason: reason,
	})
	return c.conn.Wait(context.TODO())
}

func (c *connection) Context() context.Context {
	return c.ctx
}

func (c *connection) Protocol() moqtransport.Protocol {
	return moqtransport.ProtocolQUIC
}

func (c *connection) Perspective() moqtransport.Perspective {
	return c.perspective
}
