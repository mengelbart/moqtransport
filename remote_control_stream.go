package moqtransport

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type remoteControlStream struct {
	logger *slog.Logger
	r      messageReader
	s      *Session

	// goAwayReceived is only touched from readMessages.
	goAwayReceived bool
}

func newRemoteControlStream(msg *wire.Setup, r messageReader, s *Session) *remoteControlStream {
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
			if errors.Is(err, io.EOF) {
				s.s.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "control stream closed",
				})
				return
			}
			s.s.closeOnError(err)
			return
		}
		switch msg := msg.(type) {
		case *wire.GoAwayCtrl:
			if s.goAwayReceived {
				s.s.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "duplicate GOAWAY on control stream",
				})
				return
			}
			s.goAwayReceived = true
			if err := s.s.onGoAway(msg); err != nil {
				s.s.closeWithError(err)
				return
			}
		default:
			s.s.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected control message type: %T", msg),
			})
			return
		}
	}
}
