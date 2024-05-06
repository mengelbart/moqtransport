package chat

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

type joinedRooms struct {
	trackID uint64
	st      *moqtransport.SendSubscription
	rts     []*moqtransport.ReceiveSubscription
}

type Client struct {
	session     *moqtransport.Session
	rooms       map[string]*joinedRooms
	lock        sync.Mutex
	nextTrackID uint64
}

func NewQUICClient(ctx context.Context, addr string) (*Client, error) {
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return nil, err
	}
	moqSession, err := moqtransport.NewClientSession(quicmoq.New(conn), moqtransport.IngestionDeliveryRole, true)
	if err != nil {
		return nil, err
	}
	return NewClient(moqSession)
}

func NewWebTransportClient(ctx context.Context, addr string) (*Client, error) {
	dialer := webtransport.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	_, session, err := dialer.Dial(ctx, addr, nil)
	if err != nil {
		return nil, err
	}
	moqSession, err := moqtransport.NewClientSession(webtransportmoq.New(session), moqtransport.IngestionDeliveryRole, false)
	if err != nil {
		return nil, err
	}
	return NewClient(moqSession)
}

func NewClient(p *moqtransport.Session) (*Client, error) {
	log.SetOutput(io.Discard)
	c := &Client{
		session:     p,
		rooms:       map[string]*joinedRooms{},
		lock:        sync.Mutex{},
		nextTrackID: 0,
	}
	go func() {
		for {
			a, err := c.session.ReadAnnouncement(context.Background())
			if err != nil {
				panic(err)
			}
			log.Printf("got Announcement: %v", a.Namespace())
			a.Accept()
		}
	}()
	go func() {
		for {
			var id string
			s, err := c.session.ReadSubscription(context.Background(), func(s *moqtransport.SendSubscription) error {
				parts := strings.SplitN(s.Namespace(), "/", 4)
				if len(parts) < 2 {
					return errors.New("invalid trackname")
				}
				var moq_chat string
				moq_chat, id = parts[0], parts[1]
				if moq_chat != "moq-chat" {
					return errors.New("invalid moq-chat namespace")
				}
				if _, ok := c.rooms[id]; !ok {
					return errors.New("invalid subscribe request")
				}
				return nil
			})
			if err != nil {
				continue
			}
			// TODO: Should add to a list instead of overwriting or at least end
			// previous subscriptions?
			c.rooms[id].st = s
		}
	}()
	return c, nil
}

func (c *Client) handleCatalogDeltas(roomID, username string, catalogTrack *moqtransport.ReceiveSubscription) error {
	buf := make([]byte, 64_000)
	for {
		n, err := catalogTrack.Read(buf)
		if err != nil {
			return err
		}
		delta, err := parseDelta(string(buf[:n]))
		if err != nil {
			return err
		}
		log.Printf("got catalog delta: %v", delta)
		for _, p := range delta.joined {
			if p == username {
				continue
			}
			t, err := c.session.Subscribe(context.Background(), 2, 0, fmt.Sprintf("moq-chat/%v/participant/%v", roomID, p), "", username)
			if err != nil {
				return err
			}
			go func(room, user string) {
				fmt.Printf("%v joined the chat %v\n> ", user, room)
				for {
					buf := make([]byte, 64_000)
					n, err = t.Read(buf)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Fprintf(os.Stdout, "room %v|user %v: %v\n> ", room, user, string(buf[:n]))
				}
			}(roomID, p)
		}
	}
}

func (c *Client) joinRoom(roomID, username string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.nextTrackID += 1
	c.rooms[roomID] = &joinedRooms{
		trackID: c.nextTrackID,
		st:      nil,
		rts:     []*moqtransport.ReceiveSubscription{},
	}
	if err := c.session.Announce(context.Background(), fmt.Sprintf("moq-chat/%v/participant/%v", roomID, username)); err != nil {
		return err
	}
	catalogTrack, err := c.session.Subscribe(context.Background(), 1, 0, fmt.Sprintf("moq-chat/%v", roomID), "/catalog", username)
	if err != nil {
		return err
	}
	buf := make([]byte, 64_000)
	var n int
	n, err = catalogTrack.Read(buf)
	if err != nil {
		return err
	}
	var participants *chatalog
	participants, err = parseChatalog(string(buf[:n]))
	if err != nil {
		return err
	}
	log.Printf("got catalog: %v", participants)
	for p := range participants.participants {
		if p == username {
			continue
		}
		t, err := c.session.Subscribe(context.Background(), 2, 0, fmt.Sprintf("moq-chat/%v/participant/%v", roomID, p), "", username)
		if err != nil {
			log.Fatalf("failed to subscribe to participant track: %v", err)
		}
		go func(room, user string) {
			for {
				buf := make([]byte, 64_000)
				n, err = t.Read(buf)
				if err != nil {
					log.Fatalf("failed to read from participant track: %v", err)
				}
				fmt.Fprintf(os.Stdout, "room %v|user %v: %v\n> ", room, user, string(buf[:n]))
			}
		}(roomID, p)
	}
	go c.handleCatalogDeltas(roomID, username, catalogTrack)
	return nil
}

func (c *Client) Run() error {
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "> ")
		cmd, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.HasPrefix(cmd, "join") {
			fields := strings.Fields(cmd)
			if len(fields) < 3 {
				fmt.Println("invalid join command, usage: 'join <room id> <username>'")
				continue
			}
			if err = c.joinRoom(fields[1], fields[2]); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(cmd, "msg") {
			fields := strings.Fields(cmd)
			if len(fields) < 3 {
				fmt.Println("invalid join command, usage: 'msg <room id> <msg>'")
				continue
			}
			msg, ok := strings.CutPrefix(cmd, fmt.Sprintf("msg %v", fields[1]))
			if !ok {
				fmt.Println("invalid msg command, usage: 'msg <room id> <msg>'")
				continue
			}
			if c.rooms[fields[1]].st == nil {
				fmt.Println("server not subscribed, dropping message")
				continue
			}
			w, err := c.rooms[fields[1]].st.NewObjectStream(0, 0, 0) // TODO
			if err != nil {
				fmt.Printf("failed to send object: %v\n", err)
				continue
			}
			if _, err = w.Write([]byte(strings.TrimSpace(msg))); err != nil {
				return fmt.Errorf("failed to write to room: %v", err)
			}
			if err = w.Close(); err != nil {
				return fmt.Errorf("failed to close object stream: %v", err)
			}
			continue
		}
		fmt.Println("invalid command, try 'join' or 'msg'")
	}
}
