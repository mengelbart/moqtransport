package main

import (
	"flag"
	"log"
	"time"

	"github.com/mengelbart/moqtransport"
)

func main() {
	addr := flag.String("addr", "https://localhost:8080/moq", "address to connect to")
	wt := flag.Bool("webtransport", false, "Use webtransport instead of QUIC")
	flag.Parse()

	if err := run(*addr, *wt); err != nil {
		log.Fatal(err)
	}
}

func run(addr string, wt bool) error {
	var p *moqtransport.Peer
	var err error

	if wt {
		p, err = moqtransport.DialWebTransport(addr, moqtransport.IngestionDeliveryRole)
	} else {
		p, err = moqtransport.DialQUIC(addr, moqtransport.IngestionDeliveryRole)
	}
	if err != nil {
		return err
	}

	defer p.CloseWithError(0, "closing conn")

	log.Println("webtransport connected")
	p.OnAnnouncement(moqtransport.AnnouncementHandlerFunc(func(s string) error {
		log.Printf("got announcement: %v", s)
		return nil
	}))
	p.OnSubscription(moqtransport.SubscriptionHandlerFunc(func(namespace, name string, _ *moqtransport.SendTrack) (uint64, time.Duration, error) {
		log.Printf("got subscription attempt: %v/%v", namespace, name)
		return 0, time.Duration(0), nil
	}))
	go p.Run(false)
	log.Println("subscribing")
	rt, err := p.Subscribe("clock", "second", "")
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 64_000)
	for {
		n, err := rt.Read(buf)
		if err != nil {
			panic(err)
		}
		log.Printf("got object: %v\n", string(buf[:n]))
	}
}
