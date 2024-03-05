package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func main() {
	certFile := flag.String("cert", "localhost.pem", "TLS certificate file")
	keyFile := flag.String("key", "localhost-key.pem", "TLS key file")
	addr := flag.String("addr", "localhost:8080", "listen address")
	wt := flag.Bool("webtransport", false, "Use webtransport instead of QUIC")
	flag.Parse()

	if err := run(context.Background(), *addr, *wt, *certFile, *keyFile); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, wt bool, certFile, keyFile string) error {
	tlsConfig, err := generateTLSConfigWithCertAndKey(certFile, keyFile)
	if err != nil {
		log.Printf("failed to generate TLS config from cert file and key, generating in memory certs: %v", err)
		tlsConfig = generateTLSConfig()
	}
	if wt {
		panic("TODO")
	}
	return listenQUIC(ctx, addr, tlsConfig)
}

func listenWebTransport(ctx context.Context, addr string, tlsConfig *tls.Config) error {
	wt := webtransport.Server{
		H3: http3.Server{
			Addr:       addr,
			TLSConfig:  tlsConfig,
			QuicConfig: &quic.Config{},
		},
	}
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading to webtransport failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		moqSession, err := moqtransport.NewServerSession(webtransportmoq.New(session), false)
		if err != nil {
			log.Printf("MoQ Session initialization failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		go handle(moqSession)
	})
	return wt.ListenAndServe()
}

func listenQUIC(ctx context.Context, addr string, tlsConfig *tls.Config) error {
	listener, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			return err
		}
		s, err := moqtransport.NewServerSession(quicmoq.New(conn), true)
		if err != nil {
			return err
		}
		go handle(s)
	}
}

func handle(p *moqtransport.Session) {
	go func() {
		s, err := p.ReadSubscription(context.Background())
		if err != nil {
			panic(err)
		}
		log.Printf("got subscription: %v", s)
		if fmt.Sprintf("%v/%v", s.Namespace(), s.Trackname()) != "clock/second" {
			s.Reject(moqtransport.SubscribeErrorUnknownTrack, "unknown namespace/trackname")
		}
		s.Accept()
		go func() {
			ticker := time.NewTicker(time.Second)
			for ts := range ticker.C {
				w, err := s.NewObjectStream(0, 0, 0) // TODO: Use meaningful values
				if err != nil {
					log.Println(err)
					return
				}
				if _, err := fmt.Fprintf(w, "%v", ts); err != nil {
					log.Println(err)
					return
				}
				if err := w.Close(); err != nil {
					log.Println(err)
					return
				}
			}
		}()
	}()
	if err := p.Announce(context.Background(), "clock"); err != nil {
		panic(err)
	}
}

func generateTLSConfigWithCertAndKey(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"moq-00"},
	}, nil
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
