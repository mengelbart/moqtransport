package main

import (
	"context"
	"log"

	"gitlab.lrz.de/cm/moqtransport/examples/chat"
)

func main() {
	c, err := chat.NewClient(context.Background(), "127.0.0.1:1909")
	if err != nil {
		log.Fatal(err)
	}
	c.Run()
}
