package integrationtests_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func quicServerSession(t *testing.T, ctx context.Context, listener *quic.Listener, handler moqtransport.AnnouncementHandler) *moqtransport.Session {
	conn, err := listener.Accept(ctx)
	assert.NoError(t, err)
	session := &moqtransport.Session{
		Conn:                quicmoq.New(conn),
		EnableDatagrams:     true,
		AnnouncementHandler: handler,
	}
	err = session.RunServer(ctx)
	assert.NoError(t, err)
	return session
}

func quicClientSession(t *testing.T, ctx context.Context, addr string, handler moqtransport.AnnouncementHandler) *moqtransport.Session {
	conn, err := quic.DialAddr(ctx, addr, generateTLSConfig(), &quic.Config{EnableDatagrams: true})
	assert.NoError(t, err)
	session := &moqtransport.Session{
		Conn:                quicmoq.New(conn),
		EnableDatagrams:     true,
		LocalRole:           moqtransport.IngestionDeliveryRole,
		AnnouncementHandler: handler,
	}
	err = session.RunClient()
	assert.NoError(t, err)
	return session
}

func TestIntegration(t *testing.T) {
	setup := func() (*quic.Listener, string, func()) {
		listener, err := quic.ListenAddr("localhost:0", generateTLSConfig(), &quic.Config{EnableDatagrams: true})
		assert.NoError(t, err)
		addr := fmt.Sprintf("localhost:%v", listener.Addr().(*net.UDPAddr).Port)
		return listener, addr, func() {
			assert.NoError(t, listener.Close())
		}
	}

	t.Run("setup", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		var wg sync.WaitGroup
		listener, addr, teardown := setup()
		defer teardown()
		wg.Add(1)
		sessionEstablished := make(chan struct{})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := quicServerSession(t, ctx, listener, nil)
			assert.NotNil(t, server)
			<-sessionEstablished
			assert.NoError(t, server.Close())
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := quicClientSession(t, ctx, addr, nil)
		close(sessionEstablished)
		wg.Wait()
		assert.NoError(t, client.Close())
	})

	t.Run("announce", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		var wg sync.WaitGroup
		listener, addr, teardown := setup()
		defer teardown()
		wg.Add(1)
		receivedAnnounceOK := make(chan struct{})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := quicServerSession(t, ctx, listener, nil)
			assert.NoError(t, server.Announce(ctx, "/namespace"))
			close(receivedAnnounceOK)
			assert.NoError(t, server.Close())
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := quicClientSession(t, ctx, addr, moqtransport.AnnouncementHandlerFunc(func(a *moqtransport.Announcement, arw moqtransport.AnnouncementResponseWriter) {
			assert.Equal(t, "/namespace", a.Namespace())
			arw.Accept()
		}))
		<-receivedAnnounceOK
		assert.NoError(t, client.Close())
		wg.Wait()
	})

	t.Run("announce_reject", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		var wg sync.WaitGroup
		listener, addr, teardown := setup()
		defer teardown()
		wg.Add(1)
		receivedAnnounceError := make(chan struct{})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := quicServerSession(t, ctx, listener, nil)
			err := server.Announce(ctx, "/namespace")
			assert.Error(t, err)
			assert.ErrorContains(t, err, "TEST_ERR")
			close(receivedAnnounceError)
			assert.NoError(t, server.Close())
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := quicClientSession(t, ctx, addr, moqtransport.AnnouncementHandlerFunc(func(_ *moqtransport.Announcement, arw moqtransport.AnnouncementResponseWriter) {
			arw.Reject(0, "TEST_ERR")
		}))
		<-receivedAnnounceError
		assert.NoError(t, client.Close())
		wg.Wait()
	})

	t.Run("subscribe", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		var wg sync.WaitGroup
		listener, addr, teardown := setup()
		defer teardown()
		wg.Add(1)
		receivedSubscribeOK := make(chan struct{})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := quicServerSession(t, ctx, listener, nil)
			sub, err := server.ReadSubscription(ctx, func(ss *moqtransport.SendSubscription) error { return nil })
			assert.NoError(t, err)
			assert.Equal(t, "namespace", sub.Namespace())
			assert.Equal(t, "track", sub.Trackname())
			<-receivedSubscribeOK
			assert.NoError(t, server.Close())
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := quicClientSession(t, ctx, addr, nil)
		_, err := client.Subscribe(ctx, 0, 0, "namespace", "track", "auth")
		assert.NoError(t, err)
		close(receivedSubscribeOK)
		assert.NoError(t, client.Close())
		wg.Wait()
	})

	t.Run("send_receive_objects", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		var wg sync.WaitGroup
		listener, addr, teardown := setup()
		defer teardown()
		wg.Add(1)
		receivedObject := make(chan struct{})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := quicServerSession(t, ctx, listener, nil)
			sub, err := server.ReadSubscription(ctx, func(ss *moqtransport.SendSubscription) error { return nil })
			assert.NoError(t, err)
			assert.Equal(t, "namespace", sub.Namespace())
			assert.Equal(t, "track", sub.Trackname())
			s, err := sub.NewObjectStream(0, 0, 0)
			assert.NoError(t, err)
			_, err = s.Write([]byte("hello world"))
			assert.NoError(t, err)
			assert.NoError(t, s.Close())
			<-receivedObject
			assert.NoError(t, server.Close())
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := quicClientSession(t, ctx, addr, nil)
		sub, err := client.Subscribe(ctx, 0, 0, "namespace", "track", "auth")
		assert.NoError(t, err)
		buf := make([]byte, 1500)
		n, err := sub.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, "hello world", string(buf[:n]))
		close(receivedObject)
		assert.NoError(t, client.Close())
		wg.Wait()
	})

	t.Run("unsubscribe", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		var wg sync.WaitGroup
		listener, addr, teardown := setup()
		defer teardown()
		wg.Add(1)
		receivedUnsubscribe := make(chan struct{})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := quicServerSession(t, ctx, listener, nil)
			sub, err := server.ReadSubscription(ctx, func(ss *moqtransport.SendSubscription) error { return nil })
			assert.NoError(t, err)
			assert.Equal(t, "namespace", sub.Namespace())
			assert.Equal(t, "track", sub.Trackname())
			for i := 0; i < 10; i++ {
				err = sub.NewObjectPreferDatagram(0, 0, 0, nil)
				if err != nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			assert.Error(t, err)
			assert.ErrorContains(t, err, "peer unsubscribed")
			close(receivedUnsubscribe)
			assert.NoError(t, server.Close())
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := quicClientSession(t, ctx, addr, nil)
		sub, err := client.Subscribe(ctx, 0, 0, "namespace", "track", "auth")
		assert.NoError(t, err)
		sub.Unsubscribe()
		<-receivedUnsubscribe
		assert.NoError(t, err)
		assert.NoError(t, client.Close())
		wg.Wait()
	})
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
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{tlsCert},
		NextProtos:         []string{"moq-00"},
	}
}
