package main

import (
	"context"
	"flag"
	"log"

	"github.com/mengelbart/moqtransport"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "address to connect to")
	wt := flag.Bool("webtransport", false, "Use webtransport instead of QUIC")
	flag.Parse()

	if err := run(*addr, *wt); err != nil {
		log.Fatal(err)
	}
}

func run(addr string, wt bool) error {
	var p *moqtransport.Session
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

	for {
		var a *moqtransport.Announcement
		a, err = p.ReadAnnouncement(context.Background())
		if err != nil {
			panic(err)
		}
		log.Println("got Announcement")
		if a.Namespace() == "clock" {
			a.Accept()
			break
		}
	}

	log.Println("subscribing")
	rs, err := p.Subscribe(context.Background(), 0, 0, "clock", "second", "")
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 64_000)
	for {
		n, err := rs.Read(buf)
		if err != nil {
			panic(err)
		}
		log.Printf("got object: %v\n", string(buf[:n]))
	}
}
