package chat

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type Server struct {
	chatRooms   map[string]*room
	peers       map[*moqtransport.Session]string
	nextTrackID uint64
	lock        sync.Mutex
}

func NewServer() *Server {
	s := &Server{
		chatRooms:   map[string]*room{},
		peers:       map[*moqtransport.Session]string{},
		nextTrackID: 1,
		lock:        sync.Mutex{},
	}
	return s
}

func (s *Server) ListenQUIC(ctx context.Context, addr string, tlsConfig *tls.Config) error {
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
		session, err := moqtransport.NewServerSession(quicmoq.New(conn), true)
		if err != nil {
			return err
		}
		go s.handle(session)
	}
}

func (s *Server) ListenWebTransport(ctx context.Context, addr string, tlsConfig *tls.Config) error {
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
		go s.handle(moqSession)
	})
	return wt.ListenAndServe()
}

func (s *Server) handle(p *moqtransport.Session) {
	var name string

	go func() {
		for {
			a, err := p.ReadAnnouncement(context.Background())
			if err != nil {
				panic(err)
			}
			uri := strings.SplitN(a.Namespace(), "/", 3)
			if len(uri) < 3 {
				a.Reject(0, "invalid announcement")
				continue
			}
			moq_chat, id, username := uri[0], uri[1], uri[2]
			if moq_chat != "moq-chat" {
				a.Reject(0, "invalid moq-chat namespace")
				continue
			}
			a.Accept()
			name = username
			if _, ok := s.chatRooms[id]; !ok {
				s.chatRooms[id] = newChat(id)
			}
			if err := s.chatRooms[id].join(name, p); err != nil {
				log.Println(err)
			}
			log.Printf("announcement accepted: %v", a.Namespace())
		}
	}()
	go func() {
		for {
			sub, err := p.ReadSubscription(context.Background())
			if err != nil {
				panic(err)
			}
			if len(name) == 0 {
				// Subscribe requires a username which has to be announced
				// before subscribing
				sub.Reject(moqtransport.SubscribeErrorUnknownTrack, "subscribe without prior announcement")
				continue
			}
			parts := strings.SplitN(sub.Namespace(), "/", 2)
			if len(parts) < 2 {
				sub.Reject(moqtransport.SubscribeErrorUnknownTrack, "invalid trackname")
				continue
			}
			moq_chat, id := parts[0], parts[1]
			if moq_chat != "moq-chat" {
				sub.Reject(0, "invalid moq-chat namespace")
				continue
			}
			r, ok := s.chatRooms[id]
			if !ok {
				sub.Reject(moqtransport.SubscribeErrorUnknownTrack, "unknown chat id")
				continue
			}
			if sub.Trackname() == "/catalog" {
				sub.Accept()
				go func() {
					// TODO: Improve synchronization (buffer objects before
					// subscription finished)
					time.Sleep(100 * time.Millisecond)
					s.chatRooms[id].subscribe(name, sub)
				}()
				continue
			}

			r.lock.Lock()
			log.Printf("subscribing user %v to publisher %v", name, sub.Trackname())
			r.publishers[sub.Trackname()].subscribe(name, sub)
			r.lock.Unlock()
		}
	}()
}

type chatMessage struct {
	sender  string
	content string
}

func (m *chatMessage) MarshalBinary() (data []byte, err error) {
	return []byte(fmt.Sprintf("%v says: %v\n", m.sender, m.content)), nil
}

type subscriber struct {
	name  string
	track *moqtransport.SendSubscription
}
