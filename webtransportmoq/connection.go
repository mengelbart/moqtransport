package webtransportmoq

import (
	"context"

	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/webtransport-go"
)

type webTransportConn struct {
	session *webtransport.Session
}

func New(session *webtransport.Session) moqtransport.Connection {
	return &webTransportConn{session}
}

func (c *webTransportConn) OpenStream() (moqtransport.Stream, error) {
	s, err := c.session.OpenStream()
	if err != nil {
		return nil, err
	}
	return &stream{
		qs: s,
	}, nil
}

func (c *webTransportConn) OpenStreamSync(ctx context.Context) (moqtransport.Stream, error) {
	s, err := c.session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &stream{qs: s}, nil
}

func (c *webTransportConn) OpenUniStream() (moqtransport.SendStream, error) {
	s, err := c.session.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &sendStream{stream: s}, nil
}

func (c *webTransportConn) OpenUniStreamSync(ctx context.Context) (moqtransport.SendStream, error) {
	s, err := c.session.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &sendStream{stream: s}, nil
}

func (c *webTransportConn) AcceptStream(ctx context.Context) (moqtransport.Stream, error) {
	s, err := c.session.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &stream{qs: s}, nil
}

func (c *webTransportConn) AcceptUniStream(ctx context.Context) (moqtransport.ReceiveStream, error) {
	s, err := c.session.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &receiveStream{stream: s}, nil
}

func (c *webTransportConn) SendDatagram(b []byte) error {
	return c.session.SendDatagram(b)
}

func (c *webTransportConn) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return c.session.ReceiveDatagram(ctx)
}

func (c *webTransportConn) CloseWithError(e uint64, msg string) error {
	return c.session.CloseWithError(webtransport.SessionErrorCode(e), msg)
}
