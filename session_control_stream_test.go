package moqtransport

import (
	"io"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
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

func TestRequestIDWrongParityClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	// conn is PerspectiveServer, so the peer is a client and must use
	// even request IDs.
	conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      1,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))

	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)
	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeInvalidRequestID)})

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestDuplicateRequestIDClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})

	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	first := conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	<-requests
	<-first.drained

	conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))

	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)
	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeInvalidRequestID)})

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestControlStreamFINClosesSessionWithProtocolViolation(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	<-reader.drained
	require.NotNil(t, acceptedControlStream(session))

	reader.close(io.EOF)
	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)

	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeProtocolViolation)})

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
