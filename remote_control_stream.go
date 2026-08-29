package moqtransport

import (
	"fmt"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type remoteControlStream struct {
	logger *slog.Logger
	r      controlMessageReader
	s      *Session
}

func newRemoteControlStream(msg *wire.Setup, r controlMessageReader, s *Session) *remoteControlStream {
	rcs := &remoteControlStream{
		logger: defaultLogger.With("stream", "remote_control"),
		r:      r,
		s:      s,
	}
	rcs.logger.Debug("remote control stream created", "setup", msg)
	return rcs
}

// readMessages reads from the remote control stream until it fails. It must be
// called from a goroutine tracked by the session WaitGroup.
func (s *remoteControlStream) readMessages() {
	for {
		msg, err := s.r.Read()
		if err != nil {
			s.s.handleReaderError(err)
			return
		}
		switch msg := msg.(type) {
		case *wire.GoAwayCtrl:
			s.s.onGoAway(msg)
		default:
			s.s.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected control message type: %T", msg),
			})
			return
		}
	}
}
