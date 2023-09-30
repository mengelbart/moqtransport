package main

import (
	"context"
	"log"

	"gitlab.lrz.de/cm/moqtransport"
)

func main() {
	runQUIC()
}

func runQUIC() {
	c, err := moqtransport.NewQUICClient(context.TODO(), "127.0.0.1:1909")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := c.Subscribe(""); err != nil {
		panic(err)
	}
	if err := c.Announce("catalog"); err != nil {
		panic(err)
	}
	select {}
}

func runWT() {
	c, err := moqtransport.NewWebTransportClient(context.TODO(), "127.0.0.1:1909")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := c.Subscribe(""); err != nil {
		panic(err)
	}
	if err := c.Announce("catalog"); err != nil {
		panic(err)
	}
	select {}
}
