package moqtransport

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func DialWebTransport(ctx context.Context, addr string, role uint64) (*Peer, error) {
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
	_, conn, err := d.Dial(ctx, addr, nil)
	if err != nil {
		return nil, err
	}
	wc := &webTransportConn{
		sess: conn,
	}
	return newClientPeer(ctx, wc, role, "")
}

func DialQUIC(ctx context.Context, addr string, role uint64, path string) (*Peer, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}
	conn, err := quic.DialAddr(context.TODO(), addr, tlsConf, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return nil, err
	}
	qc := &quicConn{
		conn: conn,
	}
	p, err := newClientPeer(ctx, qc, role, path)
	if err != nil {
		if errors.Is(err, errUnsupportedVersion) {
			conn.CloseWithError(SessionTerminatedErrorCode, errUnsupportedVersion.Error())
		}
		if errors.Is(err, errPathParamFromServer) {
			conn.CloseWithError(SessionTerminatedErrorCode, errPathParamFromServer.Error())
		}
		conn.CloseWithError(GenericErrorCode, "internal server error")
		return nil, err
	}
	return p, nil
}
