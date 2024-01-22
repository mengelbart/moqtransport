package chat

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/mengelbart/moqtransport"
)

type room struct {
	id           string
	participants *chatalog
	publishers   map[string]*publisher
	subscribers  map[string]*moqtransport.SendSubscription
	lock         sync.Mutex
	closeCh      chan struct{}
	closeWG      sync.WaitGroup
	ch           chan encoding.BinaryMarshaler
	subscribeCh  chan *subscriber
}

func newChat(id string) *room {
	c := &room{
		id: id,
		participants: &chatalog{
			version:      0,
			participants: map[string]struct{}{},
		},
		publishers:  map[string]*publisher{},
		subscribers: map[string]*moqtransport.SendSubscription{},
		lock:        sync.Mutex{},
		closeCh:     make(chan struct{}),
		closeWG:     sync.WaitGroup{},
		ch:          make(chan encoding.BinaryMarshaler),
		subscribeCh: make(chan *subscriber),
	}
	go c.broadcast()
	return c
}

func (r *room) join(username string, p *moqtransport.Session) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.publishers[username]; ok {
		return errors.New("username already taken")
	}
	t, err := p.Subscribe(context.Background(), 0, 0, fmt.Sprintf("moq-chat/%v", r.id), username, "")
	if err != nil {
		return err
	}
	pub := newPublisher(t)
	r.publishers[username] = pub
	r.participants.participants[username] = struct{}{}
	delta := &delta{
		joined: []string{username},
		left:   []string{},
	}
	r.ch <- delta
	return nil
}

func (r *room) subscribe(name string, t *moqtransport.SendSubscription) error {
	w, err := t.NewObjectStream(0, 0, 0) //TODO
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(r.participants.serialize()))
	if err != nil {
		return err
	}
	r.subscribeCh <- &subscriber{
		name:  name,
		track: t,
	}
	return nil
}

func (r *room) close() {
	close(r.closeCh)
	r.closeWG.Wait()
}

func (r *room) broadcast() {
	r.closeWG.Add(1)
	defer r.closeWG.Done()
	for {
		select {
		case msg := <-r.ch:
			data, err := msg.MarshalBinary()
			if err != nil {
				log.Println(err)
			}
			for _, s := range r.subscribers {
				w, err := s.NewObjectStream(0, 0, 0)
				if err != nil {
					log.Println(err)
					continue
				}
				_, err = w.Write(data)
				if err != nil {
					log.Println(err)
				}
			}

		case s := <-r.subscribeCh:
			r.subscribers[s.name] = s.track
		case <-r.closeCh:
			return
		}
	}
}
