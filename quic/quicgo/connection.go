package quicgo

import (
	"context"

	"github.com/mengelbart/moqtransport/quic"
	quicgo "github.com/quic-go/quic-go"
)

type connection struct {
	connection  *quicgo.Conn
	perspective quic.Perspective
}

func NewServer(conn *quicgo.Conn) quic.Connection {
	return New(conn, quic.PerspectiveServer)
}

func NewClient(conn *quicgo.Conn) quic.Connection {
	return New(conn, quic.PerspectiveClient)
}

func New(conn *quicgo.Conn, perspective quic.Perspective) quic.Connection {
	return &connection{conn, perspective}
}

func (c *connection) AcceptStream(ctx context.Context) (quic.Stream, error) {
	s, err := c.connection.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

func (c *connection) AcceptUniStream(ctx context.Context) (quic.ReceiveStream, error) {
	s, err := c.connection.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &ReceiveStream{
		stream: s,
	}, nil
}

func (c *connection) OpenStream() (quic.Stream, error) {
	s, err := c.connection.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

func (c *connection) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	s, err := c.connection.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

func (c *connection) OpenUniStream() (quic.SendStream, error) {
	s, err := c.connection.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &SendStream{
		stream: s,
	}, nil
}

func (c *connection) OpenUniStreamSync(ctx context.Context) (quic.SendStream, error) {
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
	return c.connection.CloseWithError(quicgo.ApplicationErrorCode(e), msg)
}

func (c *connection) Context() context.Context {
	return c.connection.Context()
}

func (c *connection) Protocol() quic.Protocol {
	return quic.ProtocolQUIC
}

func (c *connection) ApplicationProtocol() quic.ApplicationProtocol {
	return quic.ApplicationProtocol(c.connection.ConnectionState().TLS.NegotiatedProtocol)
}

func (c *connection) Perspective() quic.Perspective {
	return c.perspective
}
