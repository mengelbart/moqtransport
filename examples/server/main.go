package main

import (
	"context"
	"log"
	"time"

	"gitlab.lrz.de/cm/moqtransport"
)

func main() {
	s := moqtransport.NewQUICServer()
	s.Handle(moqtransport.PeerHandler(moqtransport.PeerHandlerFunc(peerHandler)))
	if err := s.Listen(context.TODO()); err != nil {
		log.Fatal(err)
	}
}

func peerHandler(p *moqtransport.Peer) {
	p.OnAnnouncement(func(namespace string) error {
		log.Printf("peer handler got announcment of namespace: %v", namespace)
		return nil
	})
	p.OnSubscription(func(_ *moqtransport.SendTrack) (uint64, time.Duration, error) {
		log.Println("peer handler got subscription")
		return 0, time.Duration(0), nil
	})
}
