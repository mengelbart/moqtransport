package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/quic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSubscribeRequestError(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.RequestError{
		ErrorCode:     uint64(RequestErrorCodeDoesNotExist),
		RetryInterval: 1001,
		ErrorReason:   "no such track",
	}))

	request, err := awaitSubscribe(t, result)
	assert.Nil(t, request)
	require.ErrorIs(t, err, &RequestError{Code: RequestErrorCodeDoesNotExist})
	var re *RequestError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, "no such track", re.Reason)
	assert.Nil(t, re.Redirect)
	retry, ok := re.RetryAfter()
	assert.True(t, ok)
	assert.Equal(t, time.Second, retry)
	assert.Equal(t, 0, trackCount(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscribeRequestErrorRedirect(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.RequestError{
		ErrorCode:   uint64(RequestErrorCodeRedirect),
		ErrorReason: "moved",
		Redirect: wire.Redirect{
			ConnectURI:     "https://example.com/moq",
			TrackNamespace: [][]byte{[]byte("other")},
			TrackName:      []byte("track"),
		},
	}))

	_, err = awaitSubscribe(t, result)
	var re *RequestError
	require.ErrorAs(t, err, &re)
	require.NotNil(t, re.Redirect)
	assert.Equal(t, "https://example.com/moq", re.Redirect.ConnectURI)
	assert.Equal(t, [][]byte{[]byte("other")}, re.Redirect.Namespace)
	assert.Equal(t, []byte("track"), re.Redirect.Name)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscribeRequestErrorRedirectServerWithoutURI(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.RequestError{
		ErrorCode: uint64(RequestErrorCodeRedirect),
		Redirect: wire.Redirect{
			TrackNamespace: [][]byte{[]byte("other")},
		},
	}))

	_, err = awaitSubscribe(t, result)
	var re *RequestError
	require.ErrorAs(t, err, &re)
	require.NotNil(t, re.Redirect)
	assert.Equal(t, "", re.Redirect.ConnectURI)
	assert.Equal(t, [][]byte{[]byte("other")}, re.Redirect.Namespace)
	assert.Empty(t, re.Redirect.Name)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscribeRequestErrorRedirectWithURIOnServerClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.RequestError{
		ErrorCode: uint64(RequestErrorCodeRedirect),
		Redirect: wire.Redirect{
			ConnectURI: "https://example.com/moq",
		},
	}))

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, &SessionError{Code: uint64(ErrorCodeProtocolViolation)})
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestRequestErrorFromWireNamespaceScoped(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	msg := &wire.RequestError{
		ErrorCode: uint64(RequestErrorCodeRedirect),
		Redirect: wire.Redirect{
			TrackNamespace: [][]byte{[]byte("other")},
			TrackName:      []byte("track"),
		},
	}
	re, sessErr := session.requestErrorFromWire(msg, true)
	assert.Nil(t, re)
	require.NotNil(t, sessErr)
	assert.Equal(t, uint64(ErrorCodeProtocolViolation), sessErr.Code)

	msg.Redirect.TrackName = nil
	re, sessErr = session.requestErrorFromWire(msg, true)
	assert.Nil(t, sessErr)
	require.NotNil(t, re.Redirect)
	assert.Empty(t, re.Redirect.Name)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscribeStreamClosedWithoutResponse(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.close(io.EOF)

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, ErrRequestClosed)
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscribeContextCancelled(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	cause := errors.New("cancelled by test")
	ctx, cancel := context.WithCancelCause(context.Background())
	result, _ := subscribe(t, ctx, session, conn)
	cancel(cause)

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.bidiStreamResets())
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscribeSessionClosedWhileWaiting(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, _ := subscribe(t, context.Background(), session, conn)
	session.CloseWithError(0, "closing")

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, &SessionError{Code: 0})

	goleak.VerifyNone(t)
}

func TestSubscribeOkThenStreamClosed(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribeBoundWithStream(t, session, conn, 17)

	requestStream.close(io.EOF)
	_, err = request.ReadObject(context.Background())
	assert.ErrorIs(t, err, ErrRequestClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestRequestOkBeforeSubscribeOkClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.RequestOk{}))

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, &SessionError{Code: uint64(ErrorCodeProtocolViolation)})
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestDuplicateSubscribeOkClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	_, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 18}))
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestReadObjectAfterClose(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

	require.NoError(t, request.Close())
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.bidiStreamResets())
	_, err = request.ReadObject(context.Background())
	assert.ErrorIs(t, err, ErrRequestClosed)
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func readRequestError(t *testing.T, stream *capturingStream) *wire.RequestError {
	t.Helper()
	parser, err := wire.NewParser(bytes.NewReader(stream.written()), 18, wire.StreamTypeRequest)
	require.NoError(t, err)
	msg, err := parser.Read()
	require.NoError(t, err)
	reqErr, ok := msg.(*wire.RequestError)
	require.True(t, ok, "expected *wire.RequestError, got %T", msg)
	return reqErr
}

func TestIncomingReject(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, stream := acceptSubscribe(t, conn, handler)
	request.Reject(RequestErrorCodeDoesNotExist, "unknown")
	<-stream.closed

	reqErr := readRequestError(t, stream)
	assert.Equal(t, uint64(RequestErrorCodeDoesNotExist), reqErr.ErrorCode)
	assert.Equal(t, "unknown", reqErr.ErrorReason)
	assert.Equal(t, uint64(0), reqErr.RetryInterval)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRejectRetry(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, stream := acceptSubscribe(t, conn, handler)
	request.RejectRetry(RequestErrorCodeExcessiveLoad, "busy", time.Second)
	<-stream.closed

	reqErr := readRequestError(t, stream)
	assert.Equal(t, uint64(RequestErrorCodeExcessiveLoad), reqErr.ErrorCode)
	assert.Equal(t, "busy", reqErr.ErrorReason)
	assert.Equal(t, uint64(1001), reqErr.RetryInterval)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRedirect(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, stream := acceptSubscribe(t, conn, handler)
	require.NoError(t, request.Redirect("https://example.com/moq", [][]byte{[]byte("other")}, []byte("track")))
	<-stream.closed

	reqErr := readRequestError(t, stream)
	assert.Equal(t, uint64(RequestErrorCodeRedirect), reqErr.ErrorCode)
	assert.Equal(t, "https://example.com/moq", reqErr.Redirect.ConnectURI)
	assert.Equal(t, [][]byte{[]byte("other")}, reqErr.Redirect.TrackNamespace)
	assert.Equal(t, []byte("track"), reqErr.Redirect.TrackName)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRedirectFromClient(t *testing.T) {
	conn := newTestConnectionWithPerspective(t, quic.PerspectiveClient)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, stream := acceptSubscribe(t, conn, handler)
	err = request.Redirect("https://example.com/moq", nil, nil)
	assert.ErrorIs(t, err, errRedirectURIFromClient)
	assert.Empty(t, stream.written())

	require.NoError(t, request.Redirect("", [][]byte{[]byte("other")}, nil))
	<-stream.closed
	reqErr := readRequestError(t, stream)
	assert.Equal(t, uint64(RequestErrorCodeRedirect), reqErr.ErrorCode)
	assert.Equal(t, "", reqErr.Redirect.ConnectURI)
	assert.Equal(t, [][]byte{[]byte("other")}, reqErr.Redirect.TrackNamespace)
	assert.Empty(t, reqErr.Redirect.TrackName)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
