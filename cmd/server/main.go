package main

import (
	"context"
	"log"

	"gitlab.lrz.de/cm/moqtransport/transport"
)

func main() {
	s := transport.NewQUICServer()
	s.Handle(nil)
	if err := s.Listen(context.TODO()); err != nil {
		log.Fatal(err)
	}
}
