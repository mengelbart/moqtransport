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
	return c.session.OpenStream()
}

func (c *webTransportConn) OpenStreamSync(ctx context.Context) (moqtransport.Stream, error) {
	return c.session.OpenStreamSync(ctx)
}

func (c *webTransportConn) OpenUniStream() (moqtransport.SendStream, error) {
	return c.session.OpenUniStream()
}

func (c *webTransportConn) OpenUniStreamSync(ctx context.Context) (moqtransport.SendStream, error) {
	return c.session.OpenUniStreamSync(ctx)
}

func (c *webTransportConn) AcceptStream(ctx context.Context) (moqtransport.Stream, error) {
	return c.session.AcceptStream(ctx)
}

func (c *webTransportConn) AcceptUniStream(ctx context.Context) (moqtransport.ReceiveStream, error) {
	return c.session.AcceptUniStream(ctx)
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
