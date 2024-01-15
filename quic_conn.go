package moqtransport

import (
	"context"

	"github.com/quic-go/quic-go"
)

type quicConn struct {
	conn quic.Connection
}

func (c *quicConn) OpenStream() (stream, error) {
	return c.conn.OpenStream()
}

func (c *quicConn) OpenStreamSync(ctx context.Context) (stream, error) {
	return c.conn.OpenStreamSync(ctx)
}

func (c *quicConn) OpenUniStream() (sendStream, error) {
	return c.conn.OpenUniStream()
}

func (c *quicConn) OpenUniStreamSync(ctx context.Context) (sendStream, error) {
	return c.conn.OpenUniStreamSync(ctx)
}

func (c *quicConn) AcceptStream(ctx context.Context) (stream, error) {
	return c.conn.AcceptStream(ctx)
}

func (c *quicConn) AcceptUniStream(ctx context.Context) (receiveStream, error) {
	return c.conn.AcceptUniStream(ctx)
}

func (c *quicConn) SendMessage(b []byte) error {
	return c.conn.SendDatagram(b)
}

func (c *quicConn) ReceiveMessage(ctx context.Context) ([]byte, error) {
	return c.conn.ReceiveDatagram(ctx)
}

func (c *quicConn) CloseWithError(e uint64, msg string) error {
	return c.conn.CloseWithError(quic.ApplicationErrorCode(e), msg)
}
