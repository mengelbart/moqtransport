package moqtransport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func requireContextDone(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("subscription context not cancelled")
	}
}

func TestSubscriberResetCancelsSubscription(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, reader, _ := acceptSubscribeReader(t, conn, handler)
	request.Accept(17)
	subgroup, err := request.OpenSubgroup(0, 0, 0)
	require.NoError(t, err)
	writeObject(t, subgroup, 0, "payload")

	reader.close(errTestStreamReset)
	requireContextDone(t, request.Context())
	assert.ErrorIs(t, context.Cause(request.Context()), errSubscriptionCancelled)
	assert.ErrorIs(t, context.Cause(request.Context()), errTestStreamReset)

	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.bidiStreamResets())
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.uniStreamResets())
	assert.Nil(t, sessionCloseError(session))

	_, err = request.OpenSubgroup(1, 0, 0)
	assert.ErrorIs(t, err, errSubscriptionClosed)
	assert.ErrorIs(t, request.Close(PublishDoneStatusCodeTrackEnded, ""), errSubscriptionClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubscriberResetBeforeAccept(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, reader, _ := acceptSubscribeReader(t, conn, handler)
	reader.close(errTestStreamReset)
	requireContextDone(t, request.Context())

	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.bidiStreamResets())
	assert.Nil(t, sessionCloseError(session))
	_, err = request.OpenSubgroup(0, 0, 0)
	assert.ErrorIs(t, err, errSubscriptionClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingCloseCancelsContext(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, _ := acceptSubscribe(t, conn, handler)
	request.Accept(17)
	require.NoError(t, request.Close(PublishDoneStatusCodeTrackEnded, ""))
	requireContextDone(t, request.Context())
	assert.ErrorIs(t, context.Cause(request.Context()), errSubscriptionClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingRejectCancelsContext(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, _ := acceptSubscribe(t, conn, handler)
	request.Reject(RequestErrorCodeDoesNotExist, "")
	requireContextDone(t, request.Context())
	assert.ErrorIs(t, context.Cause(request.Context()), errSubscriptionClosed)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
