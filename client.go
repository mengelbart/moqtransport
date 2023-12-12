package moqtransport

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func DialWebTransport(addr string, role Role) (*Peer, error) {
	d := webtransport.Dialer{
		RoundTripper: &http3.RoundTripper{
			DisableCompression: false,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			EnableDatagrams: false,
		},
	}
	// TODO: Handle response?
	_, conn, err := d.Dial(context.Background(), addr, nil)
	if err != nil {
		return nil, err
	}
	wc := &webTransportConn{
		sess: conn,
	}
	return newClientPeer(wc, role, nil)
}

func DialQUIC(addr string, role Role) (*Peer, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}
	conn, err := quic.DialAddr(context.TODO(), addr, tlsConf, &quic.Config{
		MaxIdleTimeout:  60 * time.Second,
		EnableDatagrams: true,
	})
	if err != nil {
		return nil, err
	}
	return DialQUICConn(conn, role)
}

func DialQUICConn(conn quic.Connection, role Role) (*Peer, error) {
	qc := &quicConn{
		conn: conn,
	}
	p, err := newClientPeer(qc, role, nil)
	if err != nil {
		_ = conn.CloseWithError(ErrorCodeGeneric, err.Error())
		return nil, err
	}
	return p, nil
}
