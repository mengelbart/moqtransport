package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type moqHandler struct {
	server        bool
	quic          bool
	addr          string
	tlsConfig     *tls.Config
	namespace     []string
	trackname     string
	publish       bool
	subscribe     bool
	nextSessionID atomic.Uint64
	publishers    map[moqtransport.Publisher]struct{}
	lock          sync.Mutex
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
		h.handle(webtransportmoq.NewServer(session))
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
			go h.handle(quicmoq.NewServer(conn))
		}
	}
}

func (h *moqHandler) getHandler(sessionID uint64, session *moqtransport.Session) moqtransport.Handler {
	return moqtransport.HandlerFunc(func(w moqtransport.ResponseWriter, r *moqtransport.Message) {
		switch r.Method {
		case moqtransport.MessageAnnounce:
			if !h.subscribe {
				log.Printf("sessionNr: %d got unexpected announcement: %v", sessionID, r.Namespace)
				w.Reject(0, "date doesn't take announcements")
				return
			}
			if !tupleEqual(r.Namespace, h.namespace) {
				log.Printf("got unexpected announcement namespace: %v, expected %v", r.Namespace, h.namespace)
				w.Reject(0, "non-matching namespace")
				return
			}
			err := w.Accept()
			if err != nil {
				log.Printf("failed to accept announcement: %v", err)
				return
			}
			if h.subscribe {
				if err := h.subscribeAndRead(session, h.namespace, h.trackname); err != nil {
					log.Printf("failed to subscribe to track: %v", err)
					return
				}
			}
		case moqtransport.MessageSubscribe:
			if !h.publish {
				w.Reject(moqtransport.ErrorCodeSubscribeTrackDoesNotExist, "endpoint does not publish any tracks")
				return
			}
			if !tupleEqual(r.Namespace, h.namespace) || (r.Track != h.trackname) {
				w.Reject(moqtransport.ErrorCodeSubscribeTrackDoesNotExist, "unknown track")
				return
			}
			err := w.Accept()
			if err != nil {
				log.Printf("failed to accept subscription: %v", err)
				return
			}
			p, ok := w.(moqtransport.Publisher)
			if !ok {
				log.Printf("subscription response writer does not implement publisher?")
			}
			datePublisher := &publisher{
				p:           p,
				sessionID:   sessionID,
				subscribeID: r.RequestID,
				trackAlias:  r.TrackAlias,
				session:     session,
			}
			h.lock.Lock()
			h.publishers[datePublisher] = struct{}{}
			h.lock.Unlock()
		}
	})
}

func (h *moqHandler) handle(conn moqtransport.Connection) {
	id := h.nextSessionID.Add(1)
	session := moqtransport.NewSession(conn.Protocol(), conn.Perspective(), 100)
	transport := &moqtransport.Transport{
		Conn:    conn,
		Handler: h.getHandler(id, session),
		Qlogger: qlog.NewQLOGHandler(os.Stdout, "MoQ QLOG", "MoQ QLOG", conn.Perspective().String(), moqt.Schema),
		Session: session,
	}
	err := transport.Run()
	if err != nil {
		log.Printf("MoQ Session initialization failed: %v", err)
		conn.CloseWithError(0, "session initialization error")
		return
	}
	if h.publish {
		if err := session.Announce(context.Background(), h.namespace); err != nil {
			log.Printf("faild to announce namespace '%v': %v", h.namespace, err)
			return
		}
	}
}

func (h *moqHandler) subscribeAndRead(s *moqtransport.Session, namespace []string, trackname string) error {
	rs, err := s.Subscribe(context.Background(), namespace, trackname, "")
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
	ticker := time.NewTicker(time.Second)
	groupID := 0
	for ts := range ticker.C {
		h.lock.Lock()
		for p := range h.publishers {
			sg, err := p.OpenSubgroup(uint64(groupID), 0, 0)
			if err != nil {
				log.Printf("failed to open new subgroup: %v", err)
				p.CloseWithError(moqtransport.ErrorCodeSubscribeDoneSubscriptionEnded, "")
				delete(h.publishers, p)
				continue
			}
			if _, err := sg.WriteObject(0, []byte(fmt.Sprintf("%v", ts))); err != nil {
				log.Printf("failed to write time to subgroup: %v", err)
			}
			sg.Close()
			// if err := p.SendDatagram(moqtransport.Object{
			// 	GroupID:    uint64(groupID),
			// 	SubGroupID: 0,
			// 	ObjectID:   0,
			// 	Payload:    []byte(fmt.Sprintf("%v", ts)),
			// }); err != nil {
			// 	log.Printf("failed to write time to publisher: %v", err)
			// }
		}
		h.lock.Unlock()
		groupID++
	}
}

func tupleEqual(a, b []string) bool {
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
