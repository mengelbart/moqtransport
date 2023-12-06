package chat

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mengelbart/moqtransport"
)

type joinedRooms struct {
	trackID uint64
	st      *moqtransport.SendTrack
	rts     []*moqtransport.ReceiveTrack
}

type Client struct {
	peer        *moqtransport.Peer
	rooms       map[string]*joinedRooms
	lock        sync.Mutex
	nextTrackID uint64
}

func NewQUICClient(addr string) (*Client, error) {
	p, err := moqtransport.DialQUIC(addr, 3)
	if err != nil {
		return nil, err
	}
	return NewClient(p)
}

func NewWebTransportClient(addr string) (*Client, error) {
	p, err := moqtransport.DialWebTransport(addr, 3)
	if err != nil {
		return nil, err
	}
	return NewClient(p)
}

func NewClient(p *moqtransport.Peer) (*Client, error) {
	log.SetOutput(io.Discard)
	c := &Client{
		peer:        p,
		rooms:       map[string]*joinedRooms{},
		lock:        sync.Mutex{},
		nextTrackID: 0,
	}
	c.peer.OnAnnouncement(moqtransport.AnnouncementHandlerFunc(func(s string) error {
		return nil
	}))
	c.peer.OnSubscription(moqtransport.SubscriptionHandlerFunc(func(namespace, _ string, st *moqtransport.SendTrack) (uint64, time.Duration, error) {
		parts := strings.SplitN(namespace, "/", 2)
		if len(parts) < 2 {
			return 0, 0, errors.New("invalid trackname")
		}
		moq_chat, id := parts[0], parts[1]
		if moq_chat != "moq-chat" {
			return 0, 0, errors.New("invalid moq-chat namespace")
		}
		if _, ok := c.rooms[id]; !ok {
			return 0, 0, errors.New("invalid subscribe request")
		}
		c.rooms[id].st = st
		return c.rooms[id].trackID, 0, nil
	}))
	go c.peer.Run(false)
	return c, nil
}

func (c *Client) handleCatalogDeltas(roomID, username string, catalogTrack *moqtransport.ReceiveTrack) error {
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
		for _, p := range delta.joined {
			if p == username {
				continue
			}
			t, err := c.peer.Subscribe(fmt.Sprintf("moq-chat/%v", roomID), p, username)
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
		rts:     []*moqtransport.ReceiveTrack{},
	}
	if err := c.peer.Announce(fmt.Sprintf("moq-chat/%v/participant/%v", roomID, username)); err != nil {
		return err
	}
	catalogTrack, err := c.peer.Subscribe(fmt.Sprintf("moq-chat/%v", roomID), "/catalog", username)
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
	for p := range participants.participants {
		if p == username {
			continue
		}
		t, err := c.peer.Subscribe(fmt.Sprintf("moq-chat/%v", roomID), p, username)
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
			_, err = c.rooms[fields[1]].st.Write([]byte(strings.TrimSpace(msg)))
			if err != nil {
				return fmt.Errorf("failed to write to room: %v", err)
			}
			continue
		}
		fmt.Println("invalid command, try 'join' or 'msg'")
	}
}
