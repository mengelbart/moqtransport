package quicmoq

import (
	"context"

	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/quic-go"
)

type connection struct {
	connection  quic.Connection
	perspective moqtransport.Perspective
}

func NewServer(conn quic.Connection) moqtransport.Connection {
	return New(conn, moqtransport.PerspectiveServer)
}

func NewClient(conn quic.Connection) moqtransport.Connection {
	return New(conn, moqtransport.PerspectiveClient)
}

func New(conn quic.Connection, perspective moqtransport.Perspective) moqtransport.Connection {
	return &connection{
		connection:  conn,
		perspective: perspective,
	}
}

func (c *connection) AcceptStream(ctx context.Context) (moqtransport.Stream, error) {
	s, err := c.connection.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		send: &SendStream{
			stream: s,
		},
		receive: &ReceiveStream{
			stream: s,
		},
	}, nil
}

func (c *connection) AcceptUniStream(ctx context.Context) (moqtransport.ReceiveStream, error) {
	s, err := c.connection.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &ReceiveStream{
		stream: s,
	}, nil
}

func (c *connection) OpenStream() (moqtransport.Stream, error) {
	s, err := c.connection.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Stream{
		send: &SendStream{
			stream: s,
		},
		receive: &ReceiveStream{
			stream: s,
		},
	}, nil
}

func (c *connection) OpenStreamSync(ctx context.Context) (moqtransport.Stream, error) {
	s, err := c.connection.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		send: &SendStream{
			stream: s,
		},
		receive: &ReceiveStream{
			stream: s,
		},
	}, nil
}

func (c *connection) OpenUniStream() (moqtransport.SendStream, error) {
	s, err := c.connection.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &SendStream{
		stream: s,
	}, nil
}

func (c *connection) OpenUniStreamSync(ctx context.Context) (moqtransport.SendStream, error) {
	s, err := c.connection.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &SendStream{
		stream: s,
	}, nil
}

func (c *connection) SendDatagram(b []byte) error {
	return c.connection.SendDatagram(b)
}

func (c *connection) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return c.connection.ReceiveDatagram(ctx)
}

func (c *connection) CloseWithError(e uint64, msg string) error {
	return c.connection.CloseWithError(quic.ApplicationErrorCode(e), msg)
}

func (c *connection) Context() context.Context {
	return c.connection.Context()
}

func (c *connection) Protocol() moqtransport.Protocol {
	return moqtransport.ProtocolQUIC
}

func (c *connection) Perspective() moqtransport.Perspective {
	return c.perspective
}
