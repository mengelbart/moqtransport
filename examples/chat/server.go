package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type server struct {
	addr      string
	tlsConfig *tls.Config

	sessions *sessionManager
}

func newServer(addr string, tlsConfig *tls.Config) *server {
	return &server{
		addr:      addr,
		tlsConfig: tlsConfig,
		sessions:  newSessionManager(),
	}
}

func (s *server) run() error {
	ctx := context.Background()

	listener, err := quic.ListenAddr(s.addr, s.tlsConfig, &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  time.Hour,
	})
	if err != nil {
		return err
	}
	wt := webtransport.Server{
		H3: http3.Server{
			Addr:      s.addr,
			TLSConfig: s.tlsConfig,
		},
	}
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("failed to upgrade webtransport request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		moqSession, err := moqtransport.NewTransport(
			r.Context(),
			webtransportmoq.New(session),
			true,
			false,
			moqtransport.OnAnnouncement(s.sessions),
			moqtransport.OnSubscription(s.sessions),
		)
		if err != nil {
			log.Printf("failed to run server session handshake: %v", err)
			moqSession.Close()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		go s.sessions.handle(moqSession)
	})
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			return err
		}
		switch conn.ConnectionState().TLS.NegotiatedProtocol {
		case "h3":
			go wt.ServeQUICConn(conn)
		case "moq-00":
			p, err := moqtransport.NewTransport(
				ctx,
				quicmoq.New(conn),
				true,
				false,
				moqtransport.OnAnnouncement(s.sessions),
				moqtransport.OnSubscription(s.sessions),
			)
			if err != nil {
				log.Printf("err opening moqtransport server session: %v", err)
				p.Close()
				conn.CloseWithError(0, "err opening moqtransport server session")
				continue
			}
			go s.sessions.handle(p)
		}
	}
}
