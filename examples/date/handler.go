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
	quic       bool
	addr       string
	tlsConfig  *tls.Config
	namespace  []string
	trackname  string
	publish    bool
	subscribe  bool
	publishers chan moqtransport.Publisher
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
		go h.setupDateTrack()
	}
	h.handle(conn)
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
		go h.setupDateTrack()
	}
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading to webtransport failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.handle(webtransportmoq.New(session))
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
			go h.handle(quicmoq.New(conn))
		}
	}
}

func (h *moqHandler) getHandler() moqtransport.Handler {
	return moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, r *moqtransport.Request) {
		switch r.Method {
		case moqtransport.MethodAnnounce:
			log.Printf("got unexpected announcement: %v", r.Namespace)
			w.Reject(0, "date doesn't take announcements")
		case moqtransport.MethodSubscribe:
			if !h.publish {
				w.Reject(moqtransport.SubscribeErrorTrackDoesNotExist, "endpoint does not publish any tracks")
				return
			}
			if !tupleEuqal(r.Namespace, h.namespace) || (r.Track != h.trackname) {
				w.Reject(moqtransport.SubscribeErrorTrackDoesNotExist, "unknown track")
				return
			}
			err := w.Accept()
			if err != nil {
				log.Printf("failed to accept subscription: %v", err)
				return
			}
			publisher, ok := w.(moqtransport.Publisher)
			if !ok {
				log.Printf("subscription response writer does not implement publisher?")
			}
			h.publishers <- publisher
		}
	})
}

func (h *moqHandler) handle(conn moqtransport.Connection) {
	ms, err := moqtransport.NewTransport(
		conn,
		h.server,
		h.quic,
		moqtransport.OnRequest(h.getHandler()),
	)
	if err != nil {
		log.Printf("MoQ Session initialization failed: %v", err)
		conn.CloseWithError(0, "session initialization error")
		return
	}
	if h.publish {
		if err := ms.Announce(context.Background(), h.namespace); err != nil {
			log.Printf("faild to announce namespace '%v': %v", h.namespace, err)
			conn.CloseWithError(0, "internal error")
			return
		}
	}
	if h.subscribe {
		if err := h.subscribeAndRead(ms, h.namespace, h.trackname); err != nil {
			log.Printf("failed to subscribe to track: %v", err)
			conn.CloseWithError(0, "internal error")
			return
		}
	}
}

func (h *moqHandler) subscribeAndRead(s *moqtransport.Transport, namespace []string, trackname string) error {
	rs, err := s.Subscribe(context.Background(), 0, 0, namespace, trackname, "")
	if err != nil {
		return err
	}
	go func() {
		for {
			o, err := rs.ReadObject(context.Background())
			if err != nil {
				if err == io.EOF {
					log.Printf("got last object")
					return
				}
				return
			}
			log.Printf("got object %v/%v/%v of length %v: %v\n", o.ObjectID, o.GroupID, o.SubGroupID, len(o.Payload), string(o.Payload))
		}
	}()
	return nil
}

func (h *moqHandler) setupDateTrack() {
	publishers := []moqtransport.Publisher{}
	ticker := time.NewTicker(time.Second)
	groupID := 0
	for {
		select {
		case ts := <-ticker.C:
			log.Printf("TICK: %v", ts)
			for _, p := range publishers {
				if err := p.SendDatagram(moqtransport.Object{
					GroupID:    uint64(groupID),
					SubGroupID: 0,
					ObjectID:   0,
					Payload:    []byte(fmt.Sprintf("%v", ts)),
				}); err != nil {
					panic(err)
				}
			}
		case publisher := <-h.publishers:
			log.Printf("got subscriber")
			publishers = append(publishers, publisher)
		}
		groupID++
	}
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

func tupleEuqal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, t := range a {
		if t != b[i] {
			return false
		}
	}
	return true
}
