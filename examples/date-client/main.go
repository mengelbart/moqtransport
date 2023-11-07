package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mengelbart/moqtransport"
)

func main() {
	addr := flag.String("addr", "https://localhost:1909", "address to connect to")
	wt := flag.Bool("webtransport", false, "Use webtransport instead of QUIC")
	flag.Parse()

	if err := run(*addr, *wt); err != nil {
		log.Fatal(err)
	}
}
func run(addr string, wt bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var p *moqtransport.Peer
	var err error

	if wt {
		p, err = moqtransport.DialWebTransport(ctx, addr)
	} else {
		p, err = moqtransport.DialQUIC(ctx, addr)
	}
	if err != nil {
		return err
	}

	defer p.CloseWithError(0, "closing conn")

	log.Println("webtransport connected")
	p.OnAnnouncement(func(s string) error {
		log.Printf("got announcement: %v", s)
		return nil
	})
	p.OnSubscription(func(namespace, name string, _ *moqtransport.SendTrack) (uint64, time.Duration, error) {
		log.Printf("got subscription attempt: %v/%v", namespace, name)
		return 0, time.Duration(0), nil
	})
	go func() {
		if err1 := p.Run(ctx, false); err1 != nil {
			panic(err1)
		}
	}()
	log.Println("subscribing")
	rt, err := p.Subscribe("clock", "second")
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 64_000)
	for {
		n, err := rt.Read(buf)
		if err != nil {
			panic(err)
		}
		fmt.Printf("got object: %v\n", string(buf[:n]))
	}
}
