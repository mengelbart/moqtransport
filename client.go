package moqtransport

import (
	"context"
	"crypto/tls"
	"errors"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func DialWebTransport(ctx context.Context, addr string) (*Peer, error) {
	d := webtransport.Dialer{
		RoundTripper: &http3.RoundTripper{
			DisableCompression: false,
			TLSClientConfig:    &tls.Config{},
			QuicConfig: &quic.Config{
				GetConfigForClient:               nil,
				Versions:                         nil,
				HandshakeIdleTimeout:             0,
				MaxIdleTimeout:                   1<<63 - 1,
				RequireAddressValidation:         nil,
				MaxRetryTokenAge:                 0,
				MaxTokenAge:                      0,
				TokenStore:                       nil,
				InitialStreamReceiveWindow:       0,
				MaxStreamReceiveWindow:           0,
				InitialConnectionReceiveWindow:   0,
				MaxConnectionReceiveWindow:       0,
				AllowConnectionWindowIncrease:    nil,
				MaxIncomingStreams:               0,
				MaxIncomingUniStreams:            0,
				KeepAlivePeriod:                  0,
				DisablePathMTUDiscovery:          false,
				DisableVersionNegotiationPackets: false,
				Allow0RTT:                        false,
				EnableDatagrams:                  false,
				Tracer:                           nil,
			},
			EnableDatagrams:        false,
			AdditionalSettings:     nil,
			StreamHijacker:         nil,
			UniStreamHijacker:      nil,
			Dial:                   nil,
			MaxResponseHeaderBytes: 0,
		},
		StreamReorderingTimeout: 0,
	}
	rsp, conn, err := d.Dial(context.TODO(), addr, nil)
	if err != nil {
		return nil, err
	}
	// TODO: Handle rsp?
	log.Println(rsp)
	wc := &WebTransportConn{
		sess: conn,
	}
	return newClientPeer(ctx, wc)
}

func DialQUIC(ctx context.Context, addr string) (*Peer, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}
	conn, err := quic.DialAddr(context.TODO(), addr, tlsConf, &quic.Config{
		GetConfigForClient:               nil,
		Versions:                         nil,
		HandshakeIdleTimeout:             0,
		MaxIdleTimeout:                   1<<63 - 1,
		RequireAddressValidation:         nil,
		MaxRetryTokenAge:                 0,
		MaxTokenAge:                      0,
		TokenStore:                       nil,
		InitialStreamReceiveWindow:       0,
		MaxStreamReceiveWindow:           0,
		InitialConnectionReceiveWindow:   0,
		MaxConnectionReceiveWindow:       0,
		AllowConnectionWindowIncrease:    nil,
		MaxIncomingStreams:               0,
		MaxIncomingUniStreams:            0,
		KeepAlivePeriod:                  0,
		DisablePathMTUDiscovery:          false,
		DisableVersionNegotiationPackets: false,
		Allow0RTT:                        false,
		EnableDatagrams:                  true,
		Tracer:                           nil,
	})
	if err != nil {
		return nil, err
	}
	qc := &quicConn{
		conn: conn,
	}
	p, err := newClientPeer(ctx, qc)
	if err != nil {
		if errors.Is(err, errUnsupportedVersion) {
			conn.CloseWithError(SessionTerminatedErrorCode, errUnsupportedVersion.Error())
		}
		conn.CloseWithError(GenericErrorCode, "internal server error")
		return nil, err
	}
	return p, nil
}
