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

		p.OnAnnouncement(func(namespace string) error {
			uri := strings.SplitN(namespace, "/", 3)
			if len(uri) < 3 {
				return errors.New("invalid announcement")
			}
			moq_chat, id, username := uri[0], uri[1], uri[2]
			if moq_chat != "moq-chat" {
				return errors.New("invalid moq-chat namespace")
			}
			name = username
			if _, ok := s.chatRooms[id]; !ok {
				s.chatRooms[id] = newChat(id)
			}
			return s.chatRooms[id].join(name, p)
		})

		p.OnSubscription(func(namespace, username string, t *moqtransport.SendTrack) (uint64, time.Duration, error) {
			if len(name) == 0 {
				// Subscribe requires a username which has to be announced
				// before subscribing
				return 0, 0, errors.New("subscribe without prior announcement")
			}
			parts := strings.SplitN(namespace, "/", 2)
			if len(parts) < 2 {
				return 0, 0, errors.New("invalid trackname")
			}
			moq_chat, id := parts[0], parts[1]
			if moq_chat != "moq-chat" {
				return 0, 0, errors.New("invalid moq-chat namespace")
			}
			r, ok := s.chatRooms[id]
			if !ok {
				return 0, 0, errUnknownChat
			}
			if username == "/catalog" {
				go func() {
					// TODO: Improve synchronization (buffer objects before
					// subscription finished)
					time.Sleep(100 * time.Millisecond)
					s.chatRooms[id].subscribe(name, t)
				}()
				return 0, 0, nil
			}

			r.lock.Lock()
			defer r.lock.Unlock()
			log.Printf("subscribing user %v to publisher %v", name, username)
			if err := r.publishers[username].subscribe(name, t); err != nil {
				return 0, 0, err
			}
			s.nextTrackID += 1
			return s.nextTrackID, time.Duration(0), nil
		})
		go p.Run(context.Background(), false)
		p.OnClose(p.ClosePeerConnection)
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
