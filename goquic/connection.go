package goquic

import (
	"context"

	"github.com/mengelbart/moqtransport"
	"golang.org/x/net/quic"
)

type connection struct {
	conn *quic.Conn
}

func New(conn *quic.Conn) moqtransport.Connection {
	return &connection{conn}
}

// AcceptStream implements moqtransport.Connection.
func (c *connection) AcceptStream(ctx context.Context) (moqtransport.Stream, error) {
	return c.conn.AcceptStream(ctx)
}

// AcceptUniStream implements moqtransport.Connection.
func (*connection) AcceptUniStream(context.Context) (moqtransport.ReceiveStream, error) {
	panic("unimplemented")
}

// CloseWithError implements moqtransport.Connection.
func (c *connection) CloseWithError(code uint64, reason string) error {
	c.conn.Abort(&quic.ApplicationError{
		Code:   code,
		Reason: reason,
	})
	return c.conn.Wait(context.TODO())
}

// OpenStream implements moqtransport.Connection.
func (c *connection) OpenStream() (moqtransport.Stream, error) {
	return c.OpenStreamSync(context.TODO())
}

// OpenStreamSync implements moqtransport.Connection.
func (c *connection) OpenStreamSync(ctx context.Context) (moqtransport.Stream, error) {
	return c.conn.NewStream(ctx)
}

// OpenUniStream implements moqtransport.Connection.
func (c *connection) OpenUniStream() (moqtransport.SendStream, error) {
	return c.OpenUniStreamSync(context.TODO())
}

// OpenUniStreamSync implements moqtransport.Connection.
func (c *connection) OpenUniStreamSync(ctx context.Context) (moqtransport.SendStream, error) {
	return c.conn.NewSendOnlyStream(ctx)
}

// ReceiveDatagram implements moqtransport.Connection.
func (*connection) ReceiveDatagram(context.Context) ([]byte, error) {
	panic("unimplemented")
}

// SendDatagram implements moqtransport.Connection.
func (*connection) SendDatagram([]byte) error {
	panic("unimplemented")
}
