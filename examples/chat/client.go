package chat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gitlab.lrz.de/cm/moqtransport"
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

func NewClient(ctx context.Context, addr string) (*Client, error) {
	qc, err := moqtransport.NewQUICClient(ctx, addr)
	if err != nil {
		return nil, err
	}
	c := &Client{
		peer:        qc,
		rooms:       map[string]*joinedRooms{},
		lock:        sync.Mutex{},
		nextTrackID: 0,
	}
	c.peer.OnAnnouncement(func(s string) error {
		log.Printf("got unexpected announcement: %v, ignoring...", s)
		return nil
	})
	c.peer.OnSubscription(func(trackname string, st *moqtransport.SendTrack) (uint64, time.Duration, error) {
		namespace := strings.SplitN(trackname, "/", 3)
		if len(namespace) < 2 {
			return 0, 0, errors.New("invalid trackname")
		}
		moq_chat, id := namespace[0], namespace[1]
		if moq_chat != "moq-chat" {
			return 0, 0, errors.New("invalid moq-chat namespace")
		}
		if _, ok := c.rooms[id]; !ok {
			return 0, 0, errors.New("invalid subscribe request")
		}
		c.rooms[id].st = st
		return c.rooms[id].trackID, 0, nil
	})
	return c, nil
}

func (c *Client) handleCatalogDeltas(roomID, username string, catalogTrack *moqtransport.ReceiveTrack) {
	buf := make([]byte, 64_000)
	for {
		n, err := catalogTrack.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		delta, err := parseDelta(string(buf[:n]))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("got catalog delta: %v\n", delta)
		for _, p := range delta.joined {
			if p == username {
				continue
			}
			t, err := c.peer.Subscribe(fmt.Sprintf("moq-chat/%v/%v", roomID, p))
			if err != nil {
				log.Fatal(err)
			}
			go func() {
				for {
					buf := make([]byte, 64_000)
					n, err = t.Read(buf)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Fprint(os.Stdout, string(buf[:n]))
				}
			}()
		}
	}
}

func (c *Client) joinRoom(roomID, username string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.nextTrackID += 1
	c.rooms[roomID] = &joinedRooms{
		trackID: c.nextTrackID,
		st:      nil,
		rts:     []*moqtransport.ReceiveTrack{},
	}
	if err := c.peer.Announce(fmt.Sprintf("moq-chat/%v/%v", roomID, username)); err != nil {
		log.Fatal(err)
	}
	catalogTrack, err := c.peer.Subscribe(fmt.Sprintf("moq-chat/%v", roomID))
	if err != nil {
		log.Fatal(err)
	}
	buf := make([]byte, 64_000)
	var n int
	n, err = catalogTrack.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	var participants *chatalog
	participants, err = parseChatalog(string(buf[:n]))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("got catalog: %v\n", participants)
	for p := range participants.participants {
		if p == username {
			continue
		}
		t, err := c.peer.Subscribe(fmt.Sprintf("moq-chat/%v/%v", roomID, p))
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			for {
				buf := make([]byte, 64_000)
				n, err = t.Read(buf)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Fprint(os.Stdout, string(buf[:n]))
			}
		}()
	}
	go c.handleCatalogDeltas(roomID, username, catalogTrack)
}

func (c *Client) Run() {
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "> ")
		cmd, err := r.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		if strings.HasPrefix(cmd, "join") {
			fields := strings.Fields(cmd)
			if len(fields) < 3 {
				fmt.Println("invalid join command, usage: 'join <room id> <username>'")
				continue
			}
			c.joinRoom(fields[1], fields[2])
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
				fmt.Println("invalid join command, usage: 'msg <room id> <msg>'")
				continue
			}
			_, err = c.rooms[fields[1]].st.Write([]byte(msg))
			if err != nil {
				log.Fatal(err)
			}
			continue
		}
		fmt.Println("invalid command, try 'join' or 'msg'")
	}
}
