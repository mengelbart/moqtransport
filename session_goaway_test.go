package moqtransport

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/quic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

type goAway struct {
	uri     string
	timeout time.Duration
}

// goAwayHandler records the GOAWAY messages passed to HandleGoAway.
func goAwayHandler(t *testing.T, conn *testConnection) (*MockHandler, <-chan goAway) {
	t.Helper()
	received := make(chan goAway, 1)
	handler := NewMockHandler(conn.ctrl)
	handler.EXPECT().HandleGoAway(gomock.Any(), gomock.Any()).Do(func(uri string, timeout time.Duration) {
		received <- goAway{uri, timeout}
	}).AnyTimes()
	return handler, received
}

func awaitGoAway(t *testing.T, received <-chan goAway) goAway {
	t.Helper()
	select {
	case g := <-received:
		return g
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for GOAWAY")
		return goAway{}
	}
}

func requireSessionOpen(t *testing.T, session *Session) {
	t.Helper()
	select {
	case <-session.Context().Done():
		t.Fatalf("session closed unexpectedly: %v", context.Cause(session.Context()))
	default:
	}
}

// controlStreamMessages decodes everything the session wrote on its control
// stream.
func controlStreamMessages(t *testing.T, conn *testConnection) []wire.ControlMessage {
	t.Helper()
	parser, err := wire.NewParser(bytes.NewReader(conn.controlStreamWritten()), 18, wire.StreamTypeControl)
	require.NoError(t, err)
	var msgs []wire.ControlMessage
	for {
		msg, err := parser.Read()
		if err != nil {
			return msgs
		}
		msgs = append(msgs, msg)
	}
}

func sentGoAway(t *testing.T, conn *testConnection) *wire.GoAwayCtrl {
	t.Helper()
	msgs := controlStreamMessages(t, conn)
	require.Len(t, msgs, 2)
	_, ok := msgs[0].(*wire.Setup)
	require.True(t, ok, "expected *wire.Setup first, got %T", msgs[0])
	msg, ok := msgs[1].(*wire.GoAwayCtrl)
	require.True(t, ok, "expected *wire.GoAwayCtrl, got %T", msgs[1])
	return msg
}

// requestStreamMessage decodes the single message written on a request stream.
func requestStreamMessage(t *testing.T, data []byte) wire.ControlMessage {
	t.Helper()
	parser, err := wire.NewParser(bytes.NewReader(data), 18, wire.StreamTypeRequest)
	require.NoError(t, err)
	msg, err := parser.Read()
	require.NoError(t, err)
	return msg
}

func TestGoAwayReceived(t *testing.T) {
	conn := newTestConnection(t)
	handler, received := goAwayHandler(t, conn)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{Timeout: 1500, RequestID: 1}))

	assert.Equal(t, goAway{"", 1500 * time.Millisecond}, awaitGoAway(t, received))
	requireSessionOpen(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwayWithURIReceivedByClient(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	handler, received := goAwayHandler(t, conn)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{NewSessionURI: "moqt://new", RequestID: 0}))

	assert.Equal(t, goAway{"moqt://new", 0}, awaitGoAway(t, received))
	requireSessionOpen(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwayWithURIReceivedByServerClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{NewSessionURI: "moqt://new", RequestID: 1}))

	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestDuplicateGoAwayClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	handler, received := goAwayHandler(t, conn)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{RequestID: 1}))
	awaitGoAway(t, received)
	requireSessionOpen(t, session)

	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{RequestID: 1}))
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwayWrongParityClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	// conn is PerspectiveServer, so its own request IDs are odd and the
	// peer must announce an odd request ID.
	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{RequestID: 2}))

	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)
	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeInvalidRequestID)})

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwayURITooLongClosesSession(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, setupWithPath("/path")))
	reader.feed(encodeControlMessage(t, &wire.GoAwayCtrl{
		NewSessionURI: strings.Repeat("a", maxGoAwayURILength+1),
		RequestID:     0,
	}))

	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwaySend(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	require.NoError(t, session.GoAway("moqt://new", 2*time.Second))

	msg := sentGoAway(t, conn)
	assert.Equal(t, "moqt://new", msg.NewSessionURI)
	assert.Equal(t, uint64(2000), msg.Timeout)
	// No peer request was seen, so the first client request ID is announced.
	assert.Equal(t, uint64(0), msg.RequestID)

	assert.ErrorIs(t, session.GoAway("", 0), ErrGoAwaySent)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwaySendAfterPeerRequest(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      4,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	<-requests

	require.NoError(t, session.GoAway("", 0))
	assert.Equal(t, uint64(6), sentGoAway(t, conn).RequestID)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwayWithURIFromClientFails(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	assert.ErrorIs(t, session.GoAway("moqt://new", 0), ErrGoAwayURIFromClient)
	require.Eventually(t, func() bool {
		return len(controlStreamMessages(t, conn)) == 1
	}, time.Second, time.Millisecond)
	_, ok := controlStreamMessages(t, conn)[0].(*wire.Setup)
	assert.True(t, ok)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestGoAwayAfterCloseFails(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	session.CloseWithError(0, "closing")
	assert.ErrorIs(t, session.GoAway("", 0), &SessionError{Code: 0})
	goleak.VerifyNone(t)
}

func TestGoAwayWhileSetupBlockedReturnsOnClose(t *testing.T) {
	conn := newTestConnection(t)
	conn.sendStreamWriteBlock = make(chan struct{})
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result := make(chan error, 1)
	go func() { result <- session.GoAway("", 0) }()

	session.closeWithError(&SessionError{Code: 0, Reason: "closing"})
	select {
	case err := <-result:
		assert.ErrorIs(t, err, &SessionError{Code: 0})
	case <-time.After(time.Second):
		t.Fatal("GoAway did not return after the session was closed")
	}

	close(conn.sendStreamWriteBlock)
	session.CloseWithError(0, "closing")
	assert.Len(t, controlStreamMessages(t, conn), 1)
	goleak.VerifyNone(t)
}

func TestRequestAfterGoAwayRejected(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	require.NoError(t, session.GoAway("", 0))

	_, stream := conn.acceptStreamCapturing(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	<-stream.closed

	reqErr, ok := requestStreamMessage(t, stream.written()).(*wire.RequestError)
	require.True(t, ok, "expected *wire.RequestError")
	assert.Equal(t, uint64(RequestErrorCodeGoingAway), reqErr.ErrorCode)
	requireSessionOpen(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestDuplicateRequestIDAfterGoAwayClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	require.NoError(t, session.GoAway("", 0))

	subscribe := encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	})
	_, first := conn.acceptStreamCapturing(subscribe)
	<-first.closed
	conn.acceptStream(subscribe)

	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)
	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeInvalidRequestID)})

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestRequestGoAwayReceived(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	received := make(chan goAway, 1)
	result, requestStream := subscribe(t, context.Background(), session, conn, WithGoAwayHandler(func(uri string, timeout time.Duration) {
		received <- goAway{uri, timeout}
	}))
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 1}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)

	requestStream.feed(encodeControlMessage(t, &wire.GoAwayReq{NewSessionURI: "moqt://new", Timeout: 500}))
	assert.Equal(t, goAway{"moqt://new", 500 * time.Millisecond}, awaitGoAway(t, received))

	// The subscription is unaffected.
	conn.sendDatagram(encodeDatagram(1, 0, 0, "payload"))
	obj, err := request.ReadObject(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "payload", string(readPayload(t, obj)))
	requireSessionOpen(t, session)

	requestStream.feed(encodeControlMessage(t, &wire.GoAwayReq{}))
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestRequestGoAwayBeforeResponseWithoutHandler(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.GoAwayReq{}))
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 1}))
	_, err = awaitSubscribe(t, result)
	require.NoError(t, err)
	requireSessionOpen(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestRequestGoAwayWithURIReceivedByServerClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	_, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.GoAwayReq{NewSessionURI: "moqt://new"}))
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestOutgoingRequestGoAwaySend(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 1}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)

	require.NoError(t, request.GoAway("moqt://new", time.Second))
	assert.ErrorIs(t, request.GoAway("", 0), ErrGoAwaySent)

	require.NoError(t, request.Close())
	assert.ErrorIs(t, request.GoAway("", 0), ErrRequestClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestOutgoingRequestGoAwayWithURIFromClientFails(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 1)
	assert.ErrorIs(t, request.GoAway("moqt://new", 0), ErrGoAwayURIFromClient)
	require.NoError(t, request.GoAway("", 0))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRequestGoAwaySend(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	_, stream := conn.acceptStreamCapturing(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	request := <-requests

	require.NoError(t, request.GoAway("moqt://new", time.Second))
	msg, ok := requestStreamMessage(t, stream.written()).(*wire.GoAwayReq)
	require.True(t, ok, "expected *wire.GoAwayReq")
	assert.Equal(t, "moqt://new", msg.NewSessionURI)
	assert.Equal(t, uint64(1000), msg.Timeout)

	assert.ErrorIs(t, request.GoAway("", 0), ErrGoAwaySent)

	request.Reject(RequestErrorCodeGoingAway, "gone")
	assert.ErrorIs(t, request.GoAway("", 0), errSubscriptionClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRequestGoAwayWithURIFromClientFails(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	_, stream := conn.acceptStreamCapturing(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      1,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	request := <-requests

	assert.ErrorIs(t, request.GoAway("moqt://new", 0), ErrGoAwayURIFromClient)
	assert.Empty(t, stream.written())

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRequestGoAwayWithoutHandler(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	reader := conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	<-requests

	// The first GOAWAY is consumed, only the second one is a violation.
	reader.feed(encodeControlMessage(t, &wire.GoAwayReq{Timeout: 10}))
	reader.feed(encodeControlMessage(t, &wire.GoAwayReq{}))
	requireProtocolViolation(t, session)
	var sessErr *SessionError
	require.ErrorAs(t, sessionCloseError(session), &sessErr)
	assert.Equal(t, "duplicate GOAWAY on request stream", sessErr.Reason)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRequestGoAwayReceived(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	reader := conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      0,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	request := <-requests
	received := make(chan goAway, 1)
	request.OnGoAway(func(uri string, timeout time.Duration) {
		received <- goAway{uri, timeout}
	})

	reader.feed(encodeControlMessage(t, &wire.GoAwayReq{Timeout: 10}))
	assert.Equal(t, goAway{"", 10 * time.Millisecond}, awaitGoAway(t, received))
	requireSessionOpen(t, session)

	reader.feed(encodeControlMessage(t, &wire.GoAwayReq{}))
	requireProtocolViolation(t, session)
	var sessErr *SessionError
	require.ErrorAs(t, sessionCloseError(session), &sessErr)
	assert.Equal(t, "duplicate GOAWAY on request stream", sessErr.Reason)
	<-request.Context().Done()

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
