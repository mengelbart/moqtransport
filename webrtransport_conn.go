package moqtransport

import (
	"context"

	"github.com/quic-go/webtransport-go"
)

type webTransportConn struct {
	session *webtransport.Session
}

func (c *webTransportConn) OpenStream() (Stream, error) {
	return c.session.OpenStream()
}

func (c *webTransportConn) OpenStreamSync(ctx context.Context) (Stream, error) {
	return c.session.OpenStreamSync(ctx)
}

func (c *webTransportConn) OpenUniStream() (SendStream, error) {
	return c.session.OpenUniStream()
}

func (c *webTransportConn) OpenUniStreamSync(ctx context.Context) (SendStream, error) {
	return c.session.OpenUniStreamSync(ctx)
}

func (c *webTransportConn) AcceptStream(ctx context.Context) (Stream, error) {
	return c.session.AcceptStream(ctx)
}

func (c *webTransportConn) AcceptUniStream(ctx context.Context) (ReceiveStream, error) {
	return c.session.AcceptUniStream(ctx)
}

func (c *webTransportConn) SendDatagram(b []byte) error {
	panic("SendMessage is not implemented for WebTransport")
}

func (c *webTransportConn) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	panic("ReceiveMessage is not implemented for WebTransport")
}

func (c *webTransportConn) CloseWithError(e uint64, msg string) error {
	return c.session.CloseWithError(webtransport.SessionErrorCode(e), msg)
}
