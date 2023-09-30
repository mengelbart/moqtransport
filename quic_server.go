package moqtransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"

	"github.com/quic-go/quic-go"
)

type PeerHandlerFunc func(*Peer)

func (h PeerHandlerFunc) Handle(p *Peer) {
	h(p)
}

type PeerHandler interface {
	Handle(*Peer)
}

type QUICServer struct {
	ph PeerHandler
}

func NewQUICServer() *QUICServer {
	return &QUICServer{}
}

func (s *QUICServer) Handle(ph PeerHandler) {
	s.ph = ph
}

func (s *QUICServer) Listen(ctx context.Context) error {
	listener, err := quic.ListenAddr("127.0.0.1:1909", generateTLSConfig(), nil)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept(context.TODO())
		if err != nil {
			return err
		}
		qc := &quicConn{
			conn: conn,
		}
		peer, err := newServerPeer(ctx, qc)
		if err != nil {
			return err
		}
		// TODO: This should probably be a map keyed by the MoQ-URI the request
		// is targeting
		if s.ph != nil {
			s.ph.Handle(peer)
		}
		go peer.runServerPeer(ctx)
		// TODO: Manage all peers for things like rooms?
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"moq-00"},
	}
}
