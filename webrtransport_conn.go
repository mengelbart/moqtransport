package moqtransport

import (
	"context"

	"github.com/quic-go/webtransport-go"
)

type webTransportConn struct {
	sess *webtransport.Session
}

func (c *webTransportConn) OpenStream() (stream, error) {
	return c.sess.OpenStream()
}

func (c *webTransportConn) OpenStreamSync(ctx context.Context) (stream, error) {
	return c.sess.OpenStreamSync(ctx)
}

func (c *webTransportConn) OpenUniStream() (sendStream, error) {
	return c.sess.OpenUniStream()
}

func (c *webTransportConn) OpenUniStreamSync(ctx context.Context) (sendStream, error) {
	return c.sess.OpenUniStreamSync(ctx)
}

func (c *webTransportConn) AcceptStream(ctx context.Context) (stream, error) {
	return c.sess.AcceptStream(ctx)
}

func (c *webTransportConn) AcceptUniStream(ctx context.Context) (receiveStream, error) {
	return c.sess.AcceptUniStream(ctx)
}

func (c *webTransportConn) ReceiveMessage(ctx context.Context) ([]byte, error) {
	panic("ReceiveMessage is not implemented for WebTransport")
}

func (c *webTransportConn) CloseWithError(e uint64, msg string) error {
	return c.sess.CloseWithError(webtransport.SessionErrorCode(e), msg)
}
