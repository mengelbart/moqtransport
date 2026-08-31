package moqtransport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func sessionCloseError(s *Session) error {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	return s.closeErr
}

func acceptedControlStream(s *Session) *remoteControlStream {
	s.controlStreamLock.Lock()
	defer s.controlStreamLock.Unlock()
	return s.remoteControlStream
}

// A session has exactly one remote control stream. A second one from the peer
// is a protocol violation and must not replace the first.
func TestDuplicateControlStreamClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	first := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	<-first.drained
	accepted := acceptedControlStream(session)
	require.NotNil(t, accepted)

	conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)

	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeProtocolViolation)})
	assert.Same(t, accepted, acceptedControlStream(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
