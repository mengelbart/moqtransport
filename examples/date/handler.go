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
	// Create a typed handler
	typedHandler := &dateTypedHandler{
		h:         h,
		sessionID: sessionID,
		session:   session,
	}
	// Adapt it to work with the existing Handler interface
	return moqtransport.NewHandlerAdapter(typedHandler)
}

// dateTypedHandler implements TypedHandler for the date example
type dateTypedHandler struct {
	h         *moqHandler
	sessionID uint64
	session   *moqtransport.Session
}

// HandleTyped processes typed messages with full access to all fields
func (dh *dateTypedHandler) HandleTyped(w moqtransport.ResponseWriter, msg moqtransport.TypedMessage) {
	switch m := msg.(type) {
	case *moqtransport.AnnounceMessage:
		if !dh.h.subscribe {
			log.Printf("sessionNr: %d got unexpected announcement: %s", dh.sessionID, m.TrackNamespace)
			if arw, ok := w.(moqtransport.AnnounceResponseWriter); ok {
				arw.RejectDetailed(moqtransport.AnnounceErrorGeneric, "date doesn't take announcements")
			} else {
				w.Reject(0, "date doesn't take announcements")
			}
			return
		}
		// Convert namespace string to slice for comparison (temporary)
		namespace := []string{m.TrackNamespace}
		if !tupleEqual(namespace, dh.h.namespace) {
			log.Printf("got unexpected announcement namespace: %s, expected %v", m.TrackNamespace, dh.h.namespace)
			if arw, ok := w.(moqtransport.AnnounceResponseWriter); ok {
				arw.RejectDetailed(moqtransport.AnnounceErrorGeneric, "non-matching namespace")
			} else {
				w.Reject(0, "non-matching namespace")
			}
			return
		}
		if arw, ok := w.(moqtransport.AnnounceResponseWriter); ok {
			err := arw.AcceptDetailed()
			if err != nil {
				log.Printf("failed to accept announcement: %v", err)
			}
		} else {
			w.Accept()
		}

	case *moqtransport.SubscribeMessage:
		if !dh.h.publish {
			if srw, ok := w.(moqtransport.SubscribeResponseWriter); ok {
				srw.RejectDetailed(moqtransport.SubscribeErrorTrackDoesNotExist, "endpoint does not publish any tracks")
			} else {
				w.Reject(moqtransport.ErrorCodeSubscribeTrackDoesNotExist, "endpoint does not publish any tracks")
			}
			return
		}

		// Convert namespace string to slice for comparison (temporary)
		namespace := []string{m.TrackNamespace}
		if !tupleEqual(namespace, dh.h.namespace) || (m.TrackName != dh.h.trackname) {
			log.Printf("unknown track: %s/%s", m.TrackNamespace, m.TrackName)
			if srw, ok := w.(moqtransport.SubscribeResponseWriter); ok {
				srw.RejectDetailed(moqtransport.SubscribeErrorTrackDoesNotExist, "unknown track")
			} else {
				w.Reject(moqtransport.ErrorCodeSubscribeTrackDoesNotExist, "unknown track")
			}
			return
		}

		// Log subscription details
		log.Printf("Subscribe request for %s/%s", m.TrackNamespace, m.TrackName)
		log.Printf("  Request ID: %d", m.RequestID)
		log.Printf("  Track Alias: %d", m.TrackAlias)
		log.Printf("  Priority: %d", m.SubscriberPriority)
		log.Printf("  Group Order: %v", m.GroupOrder)
		log.Printf("  Filter Type: %v", m.FilterType)

		// Accept with detailed response if available
		if srw, ok := w.(moqtransport.SubscribeResponseWriter); ok {
			err := srw.AcceptDetailed(
				moqtransport.WithGroupOrderResponse(moqtransport.GroupOrderAscending),
				moqtransport.WithContentExists(true),
			)
			if err != nil {
				log.Printf("failed to accept subscription: %v", err)
				return
			}

			// Publisher is available through the SubscribeResponseWriter
			datePublisher := &publisher{
				p:          srw,
				sessionNr:  dh.sessionID,
				requestID:  m.RequestID,
				trackAlias: m.TrackAlias,
				session:    dh.session,
			}
			dh.h.lock.Lock()
			dh.h.publishers[datePublisher] = struct{}{}
			dh.h.lock.Unlock()
		} else {
			// Fallback to basic handling
			err := w.Accept()
			if err != nil {
				log.Printf("failed to accept subscription: %v", err)
				return
			}
			p, ok := w.(moqtransport.Publisher)
			if !ok {
				log.Printf("subscription response writer does not implement publisher?")
				return
			}
			datePublisher := &publisher{
				p:          p,
				sessionNr:  dh.sessionID,
				requestID:  m.RequestID,
				trackAlias: m.TrackAlias,
				session:    dh.session,
			}
			dh.h.lock.Lock()
			dh.h.publishers[datePublisher] = struct{}{}
			dh.h.lock.Unlock()
		}

	default:
		log.Printf("Unhandled message type: %s", msg.Type())
		if rejector, ok := w.(interface{ Reject(uint64, string) error }); ok {
			rejector.Reject(0, "unsupported message type")
		}
	}
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
	if h.subscribe {
		if err := h.subscribeAndRead(session, h.namespace, h.trackname); err != nil {
			log.Printf("failed to subscribe to track: %v", err)
			conn.CloseWithError(0, "internal error")
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
