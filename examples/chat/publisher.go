package chat

import (
	"log"
	"sync"

	"gitlab.lrz.de/cm/moqtransport"
)

type publisher struct {
	track       *moqtransport.ReceiveTrack
	subscribers map[string]*moqtransport.SendTrack
	lock        sync.Mutex
	closeCh     chan struct{}
	closeWG     sync.WaitGroup
}

func newPublisher(track *moqtransport.ReceiveTrack) *publisher {
	p := &publisher{
		track:       track,
		subscribers: map[string]*moqtransport.SendTrack{},
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

func (p *publisher) subscribe(username string, track *moqtransport.SendTrack) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, ok := p.subscribers[username]
	if ok {
		return errDuplicateSubscriber
	}
	p.subscribers[username] = track
	return nil
}

func (p *publisher) broadcastMsg(msg []byte) error {
	log.Println("broadcasting message")
	p.lock.Lock()
	defer p.lock.Unlock()
	for _, s := range p.subscribers {
		_, err := s.Write(msg)
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
