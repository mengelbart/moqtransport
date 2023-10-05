package chat

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gitlab.lrz.de/cm/moqtransport"
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

func NewServer() *Server {
	s := &Server{
		chatRooms:   map[string]*room{},
		peers:       map[*moqtransport.Peer]string{},
		nextTrackID: 1,
		lock:        sync.Mutex{},
		moq:         &moqtransport.Server{},
	}
	s.moq.Handler = moqtransport.PeerHandler(moqtransport.PeerHandlerFunc(s.peerHandler()))
	return s
}

func (s *Server) Listen(ctx context.Context) error {
	return s.moq.ListenQUIC(ctx)
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

		p.OnSubscription(func(trackname string, t *moqtransport.SendTrack) (uint64, time.Duration, error) {
			if len(name) == 0 {
				// Subscribe requires a username which has to be announced
				// before subscribing
				return 0, 0, errors.New("subscribe without prior announcement")
			}
			namespace := strings.SplitN(trackname, "/", 3)
			if len(namespace) < 2 {
				return 0, 0, errors.New("invalid trackname")
			}
			moq_chat, id := namespace[0], namespace[1]
			if moq_chat != "moq-chat" {
				return 0, 0, errors.New("invalid moq-chat namespace")
			}
			r, ok := s.chatRooms[id]
			if !ok {
				return 0, 0, errUnknownChat
			}
			if len(namespace) == 2 {
				go func() {
					// TODO: Improve synchronization (buffer objects before
					// subscription finished)
					time.Sleep(100 * time.Millisecond)
					s.chatRooms[id].subscribe(name, t)
				}()
				return 0, 0, nil
			}

			username := namespace[2]
			r.lock.Lock()
			defer r.lock.Unlock()
			log.Printf("subscribing user %v to publisher %v", name, username)
			if err := r.publishers[username].subscribe(s.peers[p], t); err != nil {
				return 0, 0, err
			}
			s.nextTrackID += 1
			return s.nextTrackID, time.Duration(0), nil
		})
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
