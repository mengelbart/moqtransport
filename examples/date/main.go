package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
)

func main() {
	certFile := flag.String("cert", "localhost.pem", "TLS certificate file")
	keyFile := flag.String("key", "localhost-key.pem", "TLS key file")
	addr := flag.String("addr", "localhost:8080", "listen address")
	server := flag.Bool("server", false, "run as server")
	publish := flag.Bool("publish", false, "publish a date track")
	subscribe := flag.Bool("subscribe", false, "subscribe to a date track")
	wt := flag.Bool("webtransport", false, "Use webtransport instead of QUIC")
	namespace := flag.String("namespace", "clock", "Namespace to subscribe to")
	trackname := flag.String("trackname", "second", "Track to subscribe to")
	flag.Parse()

	tlsConfig, err := generateTLSConfigWithCertAndKey(*certFile, *keyFile)
	if err != nil {
		log.Printf("failed to generate TLS config from cert file and key, generating in memory certs: %v", err)
		tlsConfig = generateTLSConfig()
	}
	h := &moqHandler{
		server:     *server,
		addr:       *addr,
		tlsConfig:  tlsConfig,
		namespace:  *namespace,
		trackname:  *trackname,
		publish:    *publish,
		subscribe:  *subscribe,
		localTrack: nil,
	}
	if *server {
		if err := h.runServer(context.TODO()); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := h.runClient(context.TODO(), *wt); err != nil {
		log.Fatal(err)
	}
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
