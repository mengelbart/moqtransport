package main

import (
	"context"
	"log"

	"gitlab.lrz.de/cm/moqtransport/transport"
)

func main() {
}

func runQUIC() {
	c, err := transport.NewQUICClient(context.TODO(), "127.0.0.1:1909")
	if err != nil {
		log.Fatal(err)
	}
	c.Subscribe("")
	c.Announce("")
	select {}
}

func runWT() {
	c, err := transport.NewWebTransportClient(context.TODO(), "127.0.0.1:1909")
	if err != nil {
		log.Fatal(err)
	}
	c.Subscribe("")
	c.Announce("")
	select {}
}
