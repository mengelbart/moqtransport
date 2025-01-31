package quicmoq

import (
	"context"

	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/quic-go"
)

type connection struct {
	connection quic.Connection
}

func New(conn quic.Connection) moqtransport.Connection {
	return &connection{conn}
}

func (c *connection) AcceptStream(ctx context.Context) (moqtransport.Stream, error) {
	return c.connection.AcceptStream(ctx)
}

func (c *connection) AcceptUniStream(ctx context.Context) (moqtransport.ReceiveStream, error) {
	return c.connection.AcceptUniStream(ctx)
}

func (c *connection) OpenStream() (moqtransport.Stream, error) {
	return c.connection.OpenStream()
}

func (c *connection) OpenStreamSync(ctx context.Context) (moqtransport.Stream, error) {
	return c.connection.OpenStreamSync(ctx)
}

func (c *connection) OpenUniStream() (moqtransport.SendStream, error) {
	return c.connection.OpenUniStream()
}

func (c *connection) OpenUniStreamSync(ctx context.Context) (moqtransport.SendStream, error) {
	return c.connection.OpenUniStreamSync(ctx)
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
