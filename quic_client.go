package moqtransport

import (
	"context"
	"crypto/tls"

	"github.com/quic-go/quic-go"
)

type QUICClient struct {
}

func NewQUICClient(ctx context.Context, addr string) (*Peer, error) {
	qc := &QUICClient{}
	return qc.connect(ctx, addr)
}

func (c *QUICClient) connect(ctx context.Context, addr string) (*Peer, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}
	conn, err := quic.DialAddr(context.TODO(), addr, tlsConf, nil)
	if err != nil {
		return nil, err
	}
	qc := &QUICConn{
		conn: conn,
	}
	return NewClientPeer(ctx, qc)
}
