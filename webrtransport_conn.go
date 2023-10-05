package moqtransport

import (
	"context"

	"github.com/quic-go/webtransport-go"
)

type WebTransportConn struct {
	sess *webtransport.Session
}

func (c *WebTransportConn) OpenStream() (stream, error) {
	return c.sess.OpenStream()
}

func (c *WebTransportConn) OpenStreamSync(ctx context.Context) (stream, error) {
	return c.sess.OpenStreamSync(ctx)
}

func (c *WebTransportConn) OpenUniStream() (sendStream, error) {
	return c.sess.OpenUniStream()
}

func (c *WebTransportConn) OpenUniStreamSync(ctx context.Context) (sendStream, error) {
	return c.sess.OpenUniStreamSync(ctx)
}

func (c *WebTransportConn) AcceptStream(ctx context.Context) (stream, error) {
	return c.sess.AcceptStream(ctx)
}

func (c *WebTransportConn) AcceptUniStream(ctx context.Context) (readStream, error) {
	return c.sess.AcceptUniStream(ctx)
}

func (c *WebTransportConn) ReceiveMessage(ctx context.Context) ([]byte, error) {
	panic("ReceiveMessage is not implemented for WebTransport")
}

func (c *WebTransportConn) CloseWithError(e uint64, msg string) error {
	return c.sess.CloseWithError(webtransport.SessionErrorCode(e), msg)
}
