package transport

import (
	"context"
	"crypto/tls"

	"github.com/quic-go/quic-go"
)

type QUICConn struct {
	conn quic.Connection
}

func (c *QUICConn) OpenStreamSync(ctx context.Context) (Stream, error) {
	return c.conn.OpenStreamSync(ctx)
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

type QUICClient struct {
}

func NewQUICClient() *QUICClient {
	return &QUICClient{}
}

func (c *QUICClient) Connect(ctx context.Context, addr string) error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}
	conn, err := quic.DialAddr(context.TODO(), addr, tlsConf, nil)
	if err != nil {
		return err
	}
	qc := &QUICConn{
		conn: conn,
	}
	_, err = NewClientPeer(ctx, qc)
	if err != nil {
		return err
	}
	return nil
}
