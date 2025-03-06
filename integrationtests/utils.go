package integrationtests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
)

func connect(t *testing.T) (server, client quic.Connection, cancel func()) {
	tlsConfig, err := generateTLSConfig()
	assert.NoError(t, err)
	listener, err := quic.ListenAddr("localhost:4242", tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	assert.NoError(t, err)

	clientConn, err := quic.DialAddr(context.Background(), "localhost:4242", &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}, &quic.Config{
		EnableDatagrams: true,
	})
	assert.NoError(t, err)

	serverConn, err := listener.Accept(context.Background())
	assert.NoError(t, err)

	return serverConn, clientConn, func() {
		listener.Close()
		assert.NoError(t, clientConn.CloseWithError(0, ""))
		assert.NoError(t, serverConn.CloseWithError(0, ""))
	}
}

func setup(t *testing.T, sConn, cConn quic.Connection, handler moqtransport.Handler) (
	serverTransport *moqtransport.Transport,
	serverSession *moqtransport.Session,
	clientTransport *moqtransport.Transport,
	clientSession *moqtransport.Session,
	cancel func(),
) {
	str := &moqtransport.Transport{
		Conn:                  quicmoq.NewServer(sConn),
		InitialMaxSubscribeID: 100,
		Handler:               handler,
	}
	st, err := str.NewSession(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, st)
	defer str.Close()

	ctr := &moqtransport.Transport{
		Conn:                  quicmoq.NewClient(cConn),
		InitialMaxSubscribeID: 100,
		Handler:               nil,
	}
	ct, err := ctr.NewSession(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, ct)
	defer ctr.Close()

	return str, st, ctr, ct, func() {
		assert.NoError(t, str.Close())
		assert.NoError(t, ctr.Close())
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"moq-00", "h3"},
	}, nil
}
