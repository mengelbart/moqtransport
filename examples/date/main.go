package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

const (
	appName = "date"
)

var usg = `%s acts either as a MoQ server or client with a track
with periodic timestamp data every second.
In both cases it can publish or subscribe to a track.

Usage of %s:
`

type options struct {
	certFile     string
	keyFile      string
	addr         string
	server       bool
	publish      bool
	subscribe    bool
	webtransport bool
	namespace    string
	trackname    string
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "%s [options]\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}
	fs.StringVar(&opts.certFile, "cert", "localhost.pem", "TLS certificate file (only used for server)")
	fs.StringVar(&opts.keyFile, "key", "localhost-key.pem", "TLS key file (only used for server)")
	fs.StringVar(&opts.addr, "addr", "localhost:8080", "listen or connect address")
	fs.BoolVar(&opts.server, "server", false, "run as server")
	fs.BoolVar(&opts.publish, "publish", false, "publish a date track")
	fs.BoolVar(&opts.subscribe, "subscribe", false, "subscribe to a date track")
	fs.BoolVar(&opts.webtransport, "webtransport", false, "Use webtransport instead of QUIC (client only)")
	fs.StringVar(&opts.namespace, "namespace", "clock", "Namespace to subscribe to")
	fs.StringVar(&opts.trackname, "trackname", "second", "Track to subscribe to")
	err := fs.Parse(args[1:])
	return &opts, err
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	opts, err := parseOptions(fs, args)

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if opts.server {
		return runServer(opts)
	}
	return runClient(opts)
}

func runServer(opts *options) error {
	tlsConfig, err := generateTLSConfigWithCertAndKey(opts.certFile, opts.keyFile)
	if err != nil {
		log.Printf("failed to generate TLS config from cert file and key, generating in memory certs: %v", err)
		tlsConfig, err = generateTLSConfig()
		if err != nil {
			log.Fatal(err)
		}
	}
	h := &moqHandler{
		server:     true,
		addr:       opts.addr,
		tlsConfig:  tlsConfig,
		namespace:  []string{opts.namespace},
		trackname:  opts.trackname,
		publish:    opts.publish,
		subscribe:  opts.subscribe,
		publishers: make(chan moqtransport.Publisher),
	}
	return h.runServer(context.TODO())
}

func runClient(opts *options) error {
	h := &moqHandler{
		server:     false,
		quic:       !opts.webtransport,
		addr:       opts.addr,
		tlsConfig:  nil,
		namespace:  []string{opts.namespace},
		trackname:  opts.trackname,
		publish:    opts.publish,
		subscribe:  opts.subscribe,
		publishers: make(chan moqtransport.Publisher),
	}
	return h.runClient(context.TODO(), opts.webtransport)
}

func generateTLSConfigWithCertAndKey(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"moq-00", "h3"},
	}, nil
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

func dialQUIC(ctx context.Context, addr string) (moqtransport.Connection, error) {
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return nil, err
	}
	return quicmoq.NewClient(conn), nil
}

func dialWebTransport(ctx context.Context, addr string) (moqtransport.Connection, error) {
	dialer := webtransport.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	_, session, err := dialer.Dial(ctx, addr, nil)
	if err != nil {
		return nil, err
	}
	return webtransportmoq.NewClient(session), nil
}
