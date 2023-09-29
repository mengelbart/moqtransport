package main

import (
	"context"
	"log"

	"gitlab.lrz.de/cm/moqtransport/transport"
)

func main() {
	c := transport.NewQUICClient()
	if err := c.Connect(context.TODO(), "127.0.0.1:1909"); err != nil {
		log.Fatal(err)
	}
	c.Subscribe()
	c.Announce()
	for {
		c.Write(buffer)
	}
	select {}
}
