package main

import (
	"context"

	"gitlab.lrz.de/cm/moqtransport/examples/chat"
)

func main() {
	s := chat.NewServer()
	s.Listen(context.TODO())
}
