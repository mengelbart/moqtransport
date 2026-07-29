package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type endpoint struct {
	server        bool
	quic          bool
	addr          string
	tlsConfig     *tls.Config
	namespace     []string
	trackname     string
	publish       bool
	subscribe     bool
	nextSessionID atomic.Uint64
	publishers    map[*moqtransport.IncomingSubscribeRequest]struct{}
	lock          sync.Mutex
	largestGroup  atomic.Uint64
}

func (e *endpoint) runClient(ctx context.Context, wt bool) error {
	var conn moqtransport.Connection
	var err error
	if wt {
		conn, err = dialWebTransport(ctx, e.addr)
	} else {
		conn, err = dialQUIC(ctx, e.addr)
	}
	if err != nil {
		return err
	}
	if e.publish {
		go e.setupDateTrack()
	}
	if err = e.handle(conn); err != nil {
		return err
	}
	select {}
}

func (e *endpoint) runServer(ctx context.Context) error {
	listener, err := quic.ListenAddr(e.addr, e.tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return err
	}
	wt := webtransport.Server{
		H3: http3.Server{
			Addr:      e.addr,
			TLSConfig: e.tlsConfig,
		},
	}
	if e.publish {
		go e.setupDateTrack()
	}
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading to webtransport failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		e.handle(webtransportmoq.NewServer(session)) //nolint:errcheck
	})
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			return err
		}
		if conn.ConnectionState().TLS.NegotiatedProtocol == "h3" {
			go wt.ServeQUICConn(conn) //nolint:errcheck
		}
		if conn.ConnectionState().TLS.NegotiatedProtocol == "moq-00" {
			go e.handle(quicmoq.NewServer(conn)) //nolint:errcheck
		}
	}
}

func (e *endpoint) handle(conn moqtransport.Connection) error {
	id := e.nextSessionID.Add(1)
	session, err := moqtransport.NewSession(conn, 18, "", moqtransport.WithHandler(&handler{endpoint: e, sessionID: id}))
	if err != nil {
		return err
	}
	if e.subscribe {
		if err := e.subscribeAndRead(session, e.namespace, e.trackname); err != nil {
			return err
		}
	}
	return nil
}

func (e *endpoint) subscribeAndRead(s *moqtransport.Session, namespace []string, trackname string) error {
	ns := [][]byte{}
	for _, n := range namespace {
		ns = append(ns, []byte(n))
	}
	rs, err := s.Subscribe(context.Background(), ns, trackname)
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

func (e *endpoint) setupDateTrack() {
	ticker := time.NewTicker(time.Second)
	groupID := 0
	for ts := range ticker.C {
		e.lock.Lock()
		log.Println("tick: sending time to publishers")
		for p := range e.publishers {
			sg, err := p.OpenSubgroup(uint64(groupID), 0, 0)
			if err != nil {
				log.Printf("failed to open new subgroup: %v", err)
				// TODO: Close publisher with error
				// p.CloseWithError(uint64(moqtransport.ErrorCodeSubscribeDoneSubscriptionEnded), "") //nolint:errcheck
				delete(e.publishers, p)
				continue
			}
			log.Printf("sending time to subgroup %v of publisher %v", groupID, p)
			if _, err := sg.WriteObject(0, []byte(fmt.Sprintf("%v", ts))); err != nil {
				log.Printf("failed to write time to subgroup: %v", err)
			}
			sg.Close() //nolint:errcheck
			// if err := p.SendDatagram(moqtransport.Object{
			// 	GroupID:    uint64(groupID),
			// 	SubGroupID: 0,
			// 	ObjectID:   0,
			// 	Payload:    []byte(fmt.Sprintf("%v", ts)),
			// }); err != nil {
			// 	log.Printf("failed to write time to publisher: %v", err)
			// }
		}
		e.lock.Unlock()
		e.largestGroup.Store(uint64(groupID))
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
