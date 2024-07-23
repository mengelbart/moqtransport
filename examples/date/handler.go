package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
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

type moqHandler struct {
	server     bool
	addr       string
	tlsConfig  *tls.Config
	namespace  string
	trackname  string
	publish    bool
	subscribe  bool
	localTrack *moqtransport.LocalTrack
}

func (h *moqHandler) runClient(ctx context.Context, wt bool) error {
	var conn moqtransport.Connection
	var err error
	if wt {
		conn, err = dialWebTransport(ctx, h.addr)
	} else {
		conn, err = dialQUIC(ctx, h.addr)
	}
	if err != nil {
		return err
	}
	if h.publish {
		h.setupDateTrack(ctx)
	}
	h.handle(ctx, conn)
	select {}
}

func (h *moqHandler) runServer(ctx context.Context) error {
	listener, err := quic.ListenAddr(h.addr, h.tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return err
	}
	wt := webtransport.Server{
		H3: http3.Server{
			Addr:      h.addr,
			TLSConfig: h.tlsConfig,
		},
	}
	if h.publish {
		h.setupDateTrack(ctx)
	}
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading to webtransport failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.handle(r.Context(), webtransportmoq.New(session))
	})
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			return err
		}
		if conn.ConnectionState().TLS.NegotiatedProtocol == "h3" {
			go wt.ServeQUICConn(conn)
		}
		if conn.ConnectionState().TLS.NegotiatedProtocol == "moq-00" {
			go h.handle(ctx, quicmoq.New(conn))
		}
	}
}

func (h *moqHandler) handle(ctx context.Context, conn moqtransport.Connection) {
	ms := &moqtransport.Session{
		Conn:            conn,
		EnableDatagrams: true,
		LocalRole:       0,
		RemoteRole:      0,
		AnnouncementHandler: moqtransport.AnnouncementHandlerFunc(func(s *moqtransport.Session, a *moqtransport.Announcement, arw moqtransport.AnnouncementResponseWriter) {
			log.Printf("got unexpected announcement: %v", a.Namespace())
			arw.Reject(0, "date doesn't take announcements")
		}),
		SubscriptionHandler: moqtransport.SubscriptionHandlerFunc(func(s *moqtransport.Session, sub *moqtransport.Subscription, srw moqtransport.SubscriptionResponseWriter) {
			if !h.publish {
				srw.Reject(0, "endpoint does not publish any tracks")
				return
			}
			if sub.Namespace != h.namespace && sub.TrackName != h.trackname {
				srw.Reject(0, "unknown track")
				return
			}
			log.Printf("trying to subscribe to: %v", h.localTrack)
			srw.Accept(h.localTrack)
		}),
		Path: "",
	}
	if h.server {
		if err := ms.RunServer(ctx); err != nil {
			log.Printf("MoQ Session initialization failed: %v", err)
			ms.CloseWithError(0, "session initialization error")
			return
		}
	} else {
		if err := ms.RunClient(); err != nil {
			log.Printf("MoQ Session initialization failed: %v", err)
			ms.CloseWithError(0, "session initialization error")
			return
		}
	}
	if h.subscribe {
		if err := h.subscribeAndRead(ctx, ms, h.namespace, h.trackname); err != nil {
			log.Printf("failed to subscribe to track :%v", err)
			ms.CloseWithError(0, "internal error")
			return
		}
	}
}

func (h *moqHandler) subscribeAndRead(ctx context.Context, s *moqtransport.Session, namespace, trackname string) error {
	rs, err := s.Subscribe(context.Background(), 0, 0, namespace, trackname, "")
	if err != nil {
		return err
	}
	go func() {
		for {
			o, err := rs.ReadObject(ctx)
			if err != nil {
				if err == io.EOF {
					log.Printf("got last object, closing session")
					s.Close()
					return
				}
				return
			}
			log.Printf("got object: %v\n", string(o.Payload))
		}
	}()
	return nil
}

func (h *moqHandler) setupDateTrack(ctx context.Context) {
	h.localTrack = moqtransport.NewLocalTrack(h.namespace, h.trackname)
	go func() {
		defer h.localTrack.Close()
		ticker := time.NewTicker(time.Second)
		id := uint64(0)
		for ts := range ticker.C {
			h.localTrack.WriteObject(ctx, moqtransport.Object{
				GroupID:              id,
				ObjectID:             0,
				PublisherPriority:    0,
				ForwardingPreference: moqtransport.ObjectForwardingPreferenceStream,
				Payload:              []byte(fmt.Sprintf("%v", ts)),
			})
			id++
		}
	}()
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
	return quicmoq.New(conn), nil
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
	return webtransportmoq.New(session), nil
}
