package moqtransport

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func readPublishDone(t *testing.T, stream *capturingStream) *wire.PublishDone {
	t.Helper()
	parser, err := wire.NewParser(bytes.NewReader(stream.written()), 18, wire.StreamTypeRequest)
	require.NoError(t, err)
	msg, err := parser.Read()
	require.NoError(t, err)
	_, ok := msg.(*wire.SubscribeOk)
	require.True(t, ok, "expected SUBSCRIBE_OK, got %T", msg)
	msg, err = parser.Read()
	require.NoError(t, err)
	done, ok := msg.(*wire.PublishDone)
	require.True(t, ok, "expected PUBLISH_DONE, got %T", msg)
	return done
}

func requireStreamClosed(t *testing.T, stream *capturingStream) {
	t.Helper()
	select {
	case <-stream.closed:
	case <-time.After(time.Second):
		t.Fatal("request stream not closed")
	}
}

func TestIncomingCloseSendsPublishDone(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, requestStream := acceptSubscribe(t, conn, handler)
	request.Accept(17)

	finished, err := request.OpenSubgroup(0, 0, 0)
	require.NoError(t, err)
	writeObject(t, finished, 0, "payload")
	require.NoError(t, finished.Close())
	reset, err := request.OpenSubgroup(1, 0, 0)
	require.NoError(t, err)
	reset.Reset(StreamResetErrorCodeCancelled)
	empty, err := request.OpenSubgroup(2, 0, 0)
	require.NoError(t, err)
	require.NoError(t, empty.Close())

	require.NoError(t, request.Close(PublishDoneStatusCodeTrackEnded, "end of track"))
	requireStreamClosed(t, requestStream)
	done := readPublishDone(t, requestStream)
	assert.Equal(t, uint64(PublishDoneStatusCodeTrackEnded), done.StatusCode)
	assert.Equal(t, uint64(3), done.StreamCount)
	assert.Equal(t, "end of track", done.ErrorReason)

	_, err = request.OpenSubgroup(3, 0, 0)
	assert.ErrorIs(t, err, errSubscriptionClosed)
	assert.ErrorIs(t, request.Close(PublishDoneStatusCodeTrackEnded, ""), errSubscriptionClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingCloseWithoutStreams(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, requestStream := acceptSubscribe(t, conn, handler)
	request.Accept(17)
	require.NoError(t, request.Close(PublishDoneStatusCodeSubscriptionEnded, ""))
	requireStreamClosed(t, requestStream)
	assert.Equal(t, uint64(0), readPublishDone(t, requestStream).StreamCount)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingCloseAfterDatagrams(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, requestStream := acceptSubscribe(t, conn, handler)
	request.Accept(17)
	require.NoError(t, request.SendDatagram(0, 0, 0, false, []byte("payload")))
	require.NoError(t, request.SendDatagramStatus(1, 0, 0, ObjectStatusEndOfTrack))
	require.Len(t, conn.datagramsSent(), 2)

	require.NoError(t, request.Close(PublishDoneStatusCodeTrackEnded, ""))
	requireStreamClosed(t, requestStream)
	assert.Equal(t, uint64(0), readPublishDone(t, requestStream).StreamCount)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingCloseWithOpenSubgroup(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, requestStream := acceptSubscribe(t, conn, handler)
	request.Accept(17)
	subgroup, err := request.OpenSubgroup(0, 0, 0)
	require.NoError(t, err)

	assert.ErrorIs(t, request.Close(PublishDoneStatusCodeTrackEnded, ""), errSubgroupsOpen)
	assert.Empty(t, requestStream.written()[len(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17})):])

	require.NoError(t, subgroup.Close())
	require.NoError(t, request.Close(PublishDoneStatusCodeTrackEnded, ""))
	requireStreamClosed(t, requestStream)
	assert.Equal(t, uint64(1), readPublishDone(t, requestStream).StreamCount)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingCloseBeforeAccept(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, requestStream := acceptSubscribe(t, conn, handler)
	assert.ErrorIs(t, request.Close(PublishDoneStatusCodeTrackEnded, ""), errSubscriptionNotAccepted)
	assert.Empty(t, requestStream.written())

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingOpenSubgroupAfterReject(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, _ := acceptSubscribe(t, conn, handler)
	request.Reject(RequestErrorCodeDoesNotExist, "")
	_, err = request.OpenSubgroup(0, 0, 0)
	assert.ErrorIs(t, err, errSubscriptionClosed)
	// Only the control stream was opened.
	assert.Equal(t, 1, conn.openedUniStreams())

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func publishDone(t *testing.T, status PublishDoneStatusCode, streamCount uint64) []byte {
	t.Helper()
	return encodeControlMessage(t, &wire.PublishDone{
		StatusCode:  uint64(status),
		StreamCount: streamCount,
		ErrorReason: "reason",
	})
}

func requirePublishDone(t *testing.T, request *OutgoingSubscribeRequest, status PublishDoneStatusCode) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := request.ReadObject(ctx)
	require.ErrorIs(t, err, &PublishDone{StatusCode: status})
	var done *PublishDone
	require.ErrorAs(t, err, &done)
	assert.Equal(t, "reason", done.Reason)
}

func TestPublishDoneWithoutStreams(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 0))

	requirePublishDone(t, request, PublishDoneStatusCodeTrackEnded)
	assert.Equal(t, 0, trackCount(session))
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestPublishDoneBeforeLateStream(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithPublishDoneTimeout(time.Hour))
	require.NoError(t, err)

	request, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 2))
	requestStream.close(io.EOF)

	first := conn.acceptUniStream(encodeDataStream(t, 17, 0, 0, testObject{0, "first"}))
	assert.Equal(t, []byte("first"), readPayload(t, readObject(t, request)))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, err = request.ReadObject(ctx)
	cancel()
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	first.close(io.EOF)
	second := conn.acceptUniStream(encodeDataStream(t, 17, 1, 0, testObject{0, "second"}))
	second.close(io.EOF)
	assert.Equal(t, []byte("second"), readPayload(t, readObject(t, request)))

	requirePublishDone(t, request, PublishDoneStatusCodeTrackEnded)
	assert.Empty(t, conn.uniStreamStops())
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestPublishDoneCountsStreamBeforeBind(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithPublishDoneTimeout(time.Hour))
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	stream := conn.acceptUniStream(encodeDataStream(t, 17, 0, 0, testObject{0, "payload"}))
	require.Eventually(t, func() bool { return trackCount(session) == 1 }, time.Second, time.Millisecond)
	stream.close(io.EOF)

	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 1))

	assert.Equal(t, []byte("payload"), readPayload(t, readObject(t, request)))
	requirePublishDone(t, request, PublishDoneStatusCodeTrackEnded)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestPublishDoneTimeoutStopsOpenStreams(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithPublishDoneTimeout(50*time.Millisecond))
	require.NoError(t, err)

	request, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	conn.acceptUniStream(encodeDataStream(t, 17, 0, 0, testObject{0, "payload"}))
	assert.Equal(t, []byte("payload"), readPayload(t, readObject(t, request)))
	requestStream.feed(publishDone(t, PublishDoneStatusCodeGoingAway, 5))

	requirePublishDone(t, request, PublishDoneStatusCodeGoingAway)
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.uniStreamStops())
	assert.Equal(t, 0, trackCount(session))
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestPublishDoneTooManyStreams(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithPublishDoneTimeout(time.Hour))
	require.NoError(t, err)

	request, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 1))

	conn.acceptUniStream(encodeDataStream(t, 17, 0, 0, testObject{0, "first"}))
	assert.Equal(t, []byte("first"), readPayload(t, readObject(t, request)))
	conn.acceptUniStream(encodeDataStream(t, 17, 1, 0, testObject{0, "second"}))
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestPublishDoneBeforeSubscribeOkClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 0))

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, &SessionError{Code: uint64(ErrorCodeProtocolViolation)})
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestMessageAfterPublishDoneClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithPublishDoneTimeout(time.Hour))
	require.NoError(t, err)

	_, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 1))
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 1))
	requireProtocolViolation(t, session)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestCloseWhileAwaitingPublishDoneStreams(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithPublishDoneTimeout(time.Hour))
	require.NoError(t, err)

	request, requestStream := subscribeBoundWithStream(t, session, conn, 17)
	requestStream.feed(publishDone(t, PublishDoneStatusCodeTrackEnded, 1))

	require.NoError(t, request.Close())
	_, err = request.ReadObject(context.Background())
	assert.ErrorIs(t, err, ErrRequestClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestCloseRequestStopsOpenStreams(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)
	conn.acceptUniStream(encodeDataStream(t, 17, 0, 0, testObject{0, "payload"}))
	assert.Equal(t, []byte("payload"), readPayload(t, readObject(t, request)))

	require.NoError(t, request.Close())
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.uniStreamStops())
	assert.Nil(t, sessionCloseError(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
