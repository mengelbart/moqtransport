package chat

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mengelbart/moqtransport"
)

var (
	errUnknownChat = errors.New("unknwon chat id")
)

type Server struct {
	chatRooms   map[string]*room
	peers       map[*moqtransport.Peer]string
	nextTrackID uint64
	lock        sync.Mutex
	moq         *moqtransport.Server
}

func NewServer(tlsConfig *tls.Config) *Server {
	s := &Server{
		chatRooms:   map[string]*room{},
		peers:       map[*moqtransport.Peer]string{},
		nextTrackID: 1,
		lock:        sync.Mutex{},
		moq: &moqtransport.Server{
			Handler:   nil,
			TLSConfig: tlsConfig,
		},
	}
	s.moq.Handler = moqtransport.PeerHandler(moqtransport.PeerHandlerFunc(s.peerHandler()))
	return s
}

func (s *Server) ListenQUIC(ctx context.Context, addr string) error {
	return s.moq.ListenQUIC(ctx, addr)
}

func (s *Server) ListenWebTransport(ctx context.Context, addr string) error {
	return s.moq.ListenWebTransport(ctx, addr)
}

func (s *Server) peerHandler() moqtransport.PeerHandlerFunc {
	return func(p *moqtransport.Peer) {
		var name string

		go func() {
			for {
				a, err := p.ReadAnnouncement(context.Background())
				if err != nil {
					panic(err)
				}
				uri := strings.SplitN(a.Namespace(), "/", 3)
				if len(uri) < 3 {
					a.Reject(errors.New("invalid announcement"))
					continue
				}
				moq_chat, id, username := uri[0], uri[1], uri[2]
				if moq_chat != "moq-chat" {
					a.Reject(errors.New("invalid moq-chat namespace"))
					continue
				}
				name = username
				if _, ok := s.chatRooms[id]; !ok {
					s.chatRooms[id] = newChat(id)
				}
				if err := s.chatRooms[id].join(name, p); err != nil {
					a.Reject(err)
				}
				a.Accept()
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
					sub.Reject(errors.New("subscribe without prior announcement"))
					continue
				}
				parts := strings.SplitN(sub.Namespace(), "/", 2)
				if len(parts) < 2 {
					sub.Reject(errors.New("invalid trackname"))
					continue
				}
				moq_chat, id := parts[0], parts[1]
				if moq_chat != "moq-chat" {
					sub.Reject(errors.New("invalid moq-chat namespace"))
					continue
				}
				r, ok := s.chatRooms[id]
				if !ok {
					sub.Reject(errUnknownChat)
					continue
				}
				if sub.Trackname() == "/catalog" {
					t := sub.Accept()
					go func() {
						// TODO: Improve synchronization (buffer objects before
						// subscription finished)
						time.Sleep(100 * time.Millisecond)
						s.chatRooms[id].subscribe(name, t)
					}()
					continue
				}

				sub.SetTrackID(s.nextTrackID)
				s.nextTrackID += 1

				r.lock.Lock()
				log.Printf("subscribing user %v to publisher %v", name, sub.Trackname())
				r.publishers[sub.Trackname()].subscribe(name, sub)
				r.lock.Unlock()
			}
		}()
	}
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
	track *moqtransport.SendTrack
}
