package moqtransport

import (
	"context"
	"crypto/tls"
	"errors"

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
	conn, err := quic.DialAddr(context.TODO(), addr, tlsConf, &quic.Config{
		GetConfigForClient:               nil,
		Versions:                         nil,
		HandshakeIdleTimeout:             0,
		MaxIdleTimeout:                   0,
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
