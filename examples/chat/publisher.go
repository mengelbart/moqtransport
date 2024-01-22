package chat

import (
	"log"
	"sync"

	"github.com/mengelbart/moqtransport"
)

type publisher struct {
	track       *moqtransport.ReceiveSubscription
	subscribers map[string]*moqtransport.SendSubscription
	lock        sync.Mutex
	closeCh     chan struct{}
	closeWG     sync.WaitGroup
}

func newPublisher(track *moqtransport.ReceiveSubscription) *publisher {
	p := &publisher{
		track:       track,
		subscribers: map[string]*moqtransport.SendSubscription{},
		lock:        sync.Mutex{},
		closeCh:     make(chan struct{}),
		closeWG:     sync.WaitGroup{},
	}
	go p.broadcast()
	return p
}

func (p *publisher) close() {
	close(p.closeCh)
	p.closeWG.Wait()
}

func (p *publisher) subscribe(username string, sub *moqtransport.SendSubscription) {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, ok := p.subscribers[username]
	if ok {
		sub.Reject(errDuplicateSubscriber)
	}
	sub.Accept()
	p.subscribers[username] = sub
}

func (p *publisher) broadcastMsg(msg []byte) error {
	log.Println("broadcasting message")
	p.lock.Lock()
	defer p.lock.Unlock()
	for _, s := range p.subscribers {
		w, err := s.NewObjectStream(0, 0, 0) //TODO
		if err != nil {
			log.Printf("failed to start new object: %v", err)
		}
		_, err = w.Write(msg)
		if err != nil {
			log.Printf("error sending message to subscriber: %v", err)
			// TODO: Remove subscriber?
		}
	}
	return nil
}

func (p *publisher) broadcast() {
	p.closeWG.Add(1)
	defer p.closeWG.Done()

	ch := make(chan []byte)
	for {
		go func() {
			buf := make([]byte, 64_000) // TODO: Larger buffer and better buffer management?
			n, err := p.track.Read(buf)
			if err != nil {
				// TODO: Remove subscriber?
				panic(err)
			}
			ch <- buf[:n]
		}()
		select {
		case msg := <-ch:
			if err := p.broadcastMsg(msg); err != nil {
				panic("TODO")
			}
		case <-p.closeCh:
			return
		}
	}
}
