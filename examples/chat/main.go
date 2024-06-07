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
	"log/slog"
	"math/big"
	"os"

	"github.com/mengelbart/moqtransport"
)

func main() {
	certFile := flag.String("cert", "localhost.pem", "TLS certificate file")
	keyFile := flag.String("key", "localhost-key.pem", "TLS key file")
	addr := flag.String("addr", "localhost:8080", "listen address")
	runAsServer := flag.Bool("server", false, "if set, run as server otherwise client")
	quic := flag.Bool("quic", false, "run client in raw QUIC mode")
	flag.Parse()

	moqtransport.SetLogHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	if *runAsServer {
		if err := runServer(*addr, *certFile, *keyFile); err != nil {
			log.Fatalf("faild to run server: %v", err)
		}
		return
	}
	if err := runClient(*addr, *quic); err != nil {
		log.Panicf("faild to run client: %v", err)
	}
	log.Println("bye")
}

func runClient(addr string, quic bool) error {
	var client *Client
	var err error
	if quic {
		client, err = NewQUICClient(context.Background(), addr)
		if err != nil {
			return err
		}
	} else {
		client, err = NewWebTransportClient(context.Background(), fmt.Sprintf("https://%v/moq", addr))
		if err != nil {
			return err
		}
	}
	return client.Run()
}

func runServer(addr, certFile, keyFile string) error {
	tlsConfig, err := generateTLSConfigWithCertAndKey(certFile, keyFile)
	if err != nil {
		log.Printf("failed to generate TLS config from cert file and key, generating in memory certs: %v", err)
		tlsConfig = generateTLSConfig()
	}
	s := newServer(addr, tlsConfig)
	return s.run()
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
		NextProtos:   []string{"moq-00", "h3"},
	}
}
