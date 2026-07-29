package moqtransport

import (
	"fmt"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type remoteControlStream struct {
	logger *slog.Logger
	r      controlMessageReader
}

func newRemoteControlStream(r controlMessageReader, msg *wire.Setup) *remoteControlStream {

	rcs := &remoteControlStream{
		logger: defaultLogger.With("stream", "remote_control"),
		r:      r,
	}
	rcs.logger.Debug("remote control stream created", "setup", msg)
	go rcs.readMessages() // TODO: Close stream
	return rcs
}

func (s *remoteControlStream) readMessages() {
	for {
		msg, err := s.r.Read()
		if err != nil {
			// TODO
			panic(err)
		}
		switch msg := msg.(type) {
		case *wire.GoAway:
			// TODO
		default:
			panic(fmt.Sprintf("unexpected control message type: %T", msg))
		}
	}
}
