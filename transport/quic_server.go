package transport

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

type QUICServer struct {
}

func NewQUICServer() *QUICServer {
	return &QUICServer{}
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
		qc := &QUICConn{
			conn: conn,
		}
		_, err = NewServerPeer(ctx, qc)
		if err != nil {
			return err
		}
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
