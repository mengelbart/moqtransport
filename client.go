package moqtransport

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func DialWebTransport(addr string, role Role) (*Session, error) {
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
	session, err := newClientSession(context.Background(), wc, role, false)
	if err != nil {
		if me, ok := err.(*moqError); ok {
			_ = conn.CloseWithError(webtransport.SessionErrorCode(me.code), me.message)
		} else {
			_ = conn.CloseWithError(genericErrorErrorCode, "internal server error")
		}
		return nil, err
	}
	return session, nil
}

func DialQUIC(addr string, role Role) (*Session, error) {
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

func DialQUICConn(conn quic.Connection, role Role) (*Session, error) {
	qc := &quicConn{
		conn: conn,
	}
	session, err := newClientSession(context.Background(), qc, role, true)
	if err != nil {
		if me, ok := err.(*moqError); ok {
			_ = conn.CloseWithError(quic.ApplicationErrorCode(me.code), me.message)
		} else {
			_ = conn.CloseWithError(genericErrorErrorCode, "internal server error")
		}
		return nil, err
	}
	return session, nil
}
