// Package integrationtests runs a client and a server session over a loopback
// connection for every transport.
package integrationtests

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quic"
	"github.com/mengelbart/moqtransport/quic/quicgo"
	"github.com/mengelbart/moqtransport/quic/webtransportgo"
	quicgoquic "github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
	"github.com/stretchr/testify/require"
)

const (
	testPath    = "/moq"
	testTimeout = 5 * time.Second
)

type transport struct {
	name    string
	connect func(t *testing.T) (server, client quic.Connection)
}

func transports() []transport {
	return []transport{
		{name: "quic", connect: connectQUIC},
		{name: "webtransport", connect: connectWebTransport},
	}
}

func forEachTransport(t *testing.T, f func(t *testing.T, tr transport)) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			f(t, tr)
		})
	}
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}

func listen(t *testing.T) *quicgoquic.Listener {
	listener, err := quicgoquic.ListenAddr("localhost:0", generateTLSConfig(t), &quicgoquic.Config{
		EnableDatagrams:                  true,
		EnableStreamResetPartialDelivery: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck
	return listener
}

func connectQUIC(t *testing.T) (server, client quic.Connection) {
	listener := listen(t)
	clientConn, err := quicgoquic.DialAddr(testContext(t), listener.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{quic.MOQT18.String()},
	}, &quicgoquic.Config{
		EnableDatagrams:                  true,
		EnableStreamResetPartialDelivery: true,
	})
	require.NoError(t, err)
	serverConn, err := listener.Accept(testContext(t))
	require.NoError(t, err)
	return quicgo.NewServer(serverConn), quicgo.NewClient(clientConn)
}

func connectWebTransport(t *testing.T) (server, client quic.Connection) {
	listener := listen(t)
	sessions := make(chan *webtransport.Session, 1)
	mux := http.NewServeMux()
	wt := &webtransport.Server{
		H3: &http3.Server{
			Handler: mux,
		},
		ApplicationProtocols: []string{quic.MOQT18.String()},
	}
	mux.HandleFunc(testPath, func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		sessions <- session
	})
	t.Cleanup(func() { wt.Close() }) //nolint:errcheck
	go func() {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return
		}
		wt.ServeQUICConn(conn) //nolint:errcheck
	}()

	dialer := webtransport.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{http3.NextProtoH3},
		},
		QUICConfig: &quicgoquic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
		},
		ApplicationProtocols: []string{quic.MOQT18.String()},
	}
	port := listener.Addr().(*net.UDPAddr).Port
	_, clientSession, err := dialer.Dial(testContext(t), fmt.Sprintf("https://localhost:%d%s", port, testPath), nil)
	require.NoError(t, err)

	var serverSession *webtransport.Session
	select {
	case serverSession = <-sessions:
	case <-testContext(t).Done():
		require.FailNow(t, "timeout waiting for server WebTransport session")
	}
	return webtransportgo.NewServer(serverSession), webtransportgo.NewClient(clientSession)
}

type testHandler struct {
	onGoAway    func(string, time.Duration)
	onSubscribe func(*moqtransport.IncomingSubscribeRequest)
}

func (h *testHandler) HandleGoAway(uri string, timeout time.Duration) {
	if h.onGoAway != nil {
		h.onGoAway(uri, timeout)
	}
}

func (h *testHandler) HandleSubscribe(r *moqtransport.IncomingSubscribeRequest) {
	if h.onSubscribe != nil {
		h.onSubscribe(r)
	}
}

func setup(t *testing.T, tr transport, serverHandler, clientHandler moqtransport.Handler) (server, client *moqtransport.Session) {
	serverConn, clientConn := tr.connect(t)
	server, err := moqtransport.NewSession(serverConn, testPath, moqtransport.WithHandler(serverHandler))
	require.NoError(t, err)
	t.Cleanup(func() { server.CloseWithError(0, "") })
	client, err = moqtransport.NewSession(clientConn, testPath, moqtransport.WithHandler(clientHandler))
	require.NoError(t, err)
	t.Cleanup(func() { client.CloseWithError(0, "") })
	return server, client
}

func subscribeHandler(trackAlias uint64) (moqtransport.Handler, <-chan *moqtransport.IncomingSubscribeRequest) {
	requests := make(chan *moqtransport.IncomingSubscribeRequest, 1)
	return &testHandler{
		onSubscribe: func(r *moqtransport.IncomingSubscribeRequest) {
			r.Accept(trackAlias)
			requests <- r
		},
	}, requests
}

func waitForRequest(t *testing.T, requests <-chan *moqtransport.IncomingSubscribeRequest) *moqtransport.IncomingSubscribeRequest {
	select {
	case r := <-requests:
		return r
	case <-testContext(t).Done():
		require.FailNow(t, "timeout waiting for subscribe request")
		return nil
	}
}

func writeObject(t *testing.T, sg *moqtransport.Subgroup, objectID uint64, payload []byte) {
	w, err := sg.OpenObject(objectID, uint64(len(payload)))
	require.NoError(t, err)
	_, err = w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func readPayload(t *testing.T, o *moqtransport.Object) []byte {
	payload, err := io.ReadAll(o.Payload)
	require.NoError(t, err)
	return payload
}

func generateTLSConfig(t *testing.T) *tls.Config {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)
	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  key,
		}},
		NextProtos: []string{quic.MOQT18.String(), http3.NextProtoH3},
	}
}
