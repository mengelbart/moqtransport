package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

type clientRoom struct {
	lt  *moqtransport.LocalTrack
	rts []*moqtransport.RemoteTrack
}

type roomManager struct {
	rooms map[string]*clientRoom
	lock  sync.Mutex
}

type Client struct {
	session *moqtransport.Session
	rm      *roomManager
}

func NewQUICClient(ctx context.Context, addr string) (*Client, error) {
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}, &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  time.Hour,
	})
	if err != nil {
		return nil, err
	}
	return NewClient(quicmoq.New(conn))
}

func NewWebTransportClient(ctx context.Context, addr string) (*Client, error) {
	dialer := webtransport.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		QUICConfig: &quic.Config{
			EnableDatagrams: true,
			MaxIdleTimeout:  time.Hour,
		},
	}
	_, session, err := dialer.Dial(ctx, addr, nil)
	if err != nil {
		return nil, err
	}
	return NewClient(webtransportmoq.New(session))
}

func (r *roomManager) HandleAnnouncement(s *moqtransport.Session, a *moqtransport.Announcement, arw moqtransport.AnnouncementResponseWriter) {
	log.Printf("got Announcement: %v", a.Namespace())
	arw.Accept()
}

func NewClient(conn moqtransport.Connection) (*Client, error) {
	log.SetOutput(io.Discard)
	rm := &roomManager{
		rooms: map[string]*clientRoom{},
		lock:  sync.Mutex{},
	}
	moqSession := &moqtransport.Session{
		Conn:                conn,
		EnableDatagrams:     true,
		LocalRole:           moqtransport.RolePubSub,
		RemoteRole:          0,
		AnnouncementHandler: rm,
		SubscriptionHandler: nil,
	}
	if err := moqSession.RunClient(); err != nil {
		return nil, err
	}
	return &Client{
		session: moqSession,
		rm:      rm,
	}, nil
}

func (c *Client) handleCatalogDeltas(roomID, username string, previous *chatalog[struct{}], catalogTrack *moqtransport.RemoteTrack) error {
	for {
		o, err := catalogTrack.ReadObject(context.Background())
		if err != nil {
			return err
		}
		var update *delta
		switch o.ObjectID {
		case 0:
			var next *chatalog[struct{}]
			next, err = parseChatalog[struct{}](string(o.Payload))
			if err != nil {
				return err
			}
			update = previous.diff(next)
		default:
			update, err = parseDelta(string(o.Payload))
			if err != nil {
				return err
			}

		}

		log.Printf("got catalog delta: %v", update)
		for _, p := range update.joined {
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
					o, err := t.ReadObject(context.Background())
					if err != nil {
						log.Fatal(err)
					}
					fmt.Fprintf(os.Stdout, "room %v|user %v: %v\n> ", room, user, string(o.Payload))
				}
			}(roomID, p)
		}
	}
}

func (c *Client) joinRoom(roomID, username string) error {
	c.rm.lock.Lock()
	defer c.rm.lock.Unlock()
	lt := moqtransport.NewLocalTrack(fmt.Sprintf("moq-chat/%v/participant/%v", roomID, username), "")
	if err := c.session.AddLocalTrack(lt); err != nil {
		return err
	}
	c.rm.rooms[roomID] = &clientRoom{
		lt:  lt,
		rts: []*moqtransport.RemoteTrack{},
	}
	catalogTrack, err := c.session.Subscribe(context.Background(), 1, 0, fmt.Sprintf("moq-chat/%v", roomID), "/catalog", username)
	if err != nil {
		return err
	}
	if err = c.session.Announce(context.Background(), fmt.Sprintf("moq-chat/%v/participant/%v", roomID, username)); err != nil {
		return err
	}
	o, err := catalogTrack.ReadObject(context.Background())
	if err != nil {
		return err
	}
	var participants *chatalog[struct{}]
	participants, err = parseChatalog[struct{}](string(o.Payload))
	if err != nil {
		return err
	}
	go func() {
		if err := c.handleCatalogDeltas(roomID, username, participants, catalogTrack); err != nil {
			panic(err)
		}
	}()
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
				o, err := t.ReadObject(context.Background())
				if err != nil {
					log.Fatalf("failed to read from participant track: %v", err)
				}
				fmt.Fprintf(os.Stdout, "room %v|user %v: %v\n> ", room, user, string(o.Payload))
			}
		}(roomID, p)
	}
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
		switch {
		case strings.HasPrefix(cmd, "join"):
			fields := strings.Fields(cmd)
			if len(fields) < 3 {
				fmt.Println("invalid join command, usage: 'join <room id> <username>'")
				break
			}
			if err = c.joinRoom(fields[1], fields[2]); err != nil {
				return err
			}
		case strings.HasPrefix(cmd, "msg"):
			fields := strings.Fields(cmd)
			if len(fields) < 3 {
				fmt.Println("invalid join command, usage: 'msg <room id> <msg>'")
				break
			}
			msg, ok := strings.CutPrefix(cmd, fmt.Sprintf("msg %v", fields[1]))
			if !ok {
				fmt.Println("invalid msg command, usage: 'msg <room id> <msg>'")
				break
			}
			if c.rm.rooms[fields[1]].lt == nil {
				fmt.Println("server not subscribed, dropping message")
				break
			}
			err := c.rm.rooms[fields[1]].lt.WriteObject(context.Background(), moqtransport.Object{
				GroupID:              0,
				ObjectID:             0,
				ObjectSendOrder:      0,
				ForwardingPreference: moqtransport.ObjectForwardingPreferenceStream,
				Payload:              []byte(strings.TrimSpace(msg)),
			})
			if err != nil {
				return fmt.Errorf("failed to write to room: %v", err)
			}
		default:
			fmt.Println("invalid command, try 'join' or 'msg'")
		}
	}
}
