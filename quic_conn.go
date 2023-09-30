package moqtransport

import (
	"context"

	"github.com/quic-go/quic-go"
)

type QUICConn struct {
	conn quic.Connection
}

func (c *QUICConn) OpenStream() (Stream, error) {
	return c.conn.OpenStream()
}

func (c *QUICConn) OpenStreamSync(ctx context.Context) (Stream, error) {
	return c.conn.OpenStreamSync(ctx)
}

func (c *QUICConn) OpenUniStream() (SendStream, error) {
	return c.conn.OpenUniStream()
}

func (c *QUICConn) OpenUniStreamSync(ctx context.Context) (SendStream, error) {
	return c.conn.OpenUniStreamSync(ctx)
}

func (c *QUICConn) AcceptStream(ctx context.Context) (Stream, error) {
	return c.conn.AcceptStream(ctx)
}

func (c *QUICConn) AcceptUniStream(ctx context.Context) (ReadStream, error) {
	return c.conn.AcceptUniStream(ctx)
}

func (c *QUICConn) ReceiveMessage(ctx context.Context) ([]byte, error) {
	return c.conn.ReceiveMessage(ctx)
}
