package moqtransport

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/moqtransport/varint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

type testReceiver struct {
	objects []*Object
}

func (r *testReceiver) push(o *Object) {
	r.objects = append(r.objects, o)
}

// testObject is an object on a subgroup stream, identified by its wire object
// ID delta rather than its object ID.
type testObject struct {
	delta   uint64
	payload string
}

func encodeDataStream(t *testing.T, trackAlias, groupID, subgroupID uint64, objects ...testObject) []byte {
	t.Helper()
	var buf bytes.Buffer
	appender := wire.NewAppender(&buf, 18)
	require.NoError(t, appender.Write(wire.NewSubgroupHeader(trackAlias, groupID, subgroupID, 0)))
	for _, o := range objects {
		require.NoError(t, appender.Write(&wire.ObjectStream{
			ObjectIDDelta: o.delta,
			ObjectPayload: []byte(o.payload),
		}))
	}
	return buf.Bytes()
}

func encodeDatagram(trackAlias, groupID, objectID uint64, payload string) []byte {
	msg := &wire.ObjectDatagram{
		TrackAlias:    trackAlias,
		GroupID:       groupID,
		ObjectID:      objectID,
		ObjectPayload: []byte(payload),
	}
	return msg.AppendDatagram(varint.Append(nil, uint64(msg.Type())))
}

func readObject(t *testing.T, request *OutgoingSubscribeRequest) *Object {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	o, err := request.ReadObject(ctx)
	require.NoError(t, err)
	return o
}

func trackCount(s *Session) int {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()
	return len(s.tracks)
}

func hasTrackAlias(s *Session, trackAlias uint64) bool {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()
	entry, ok := s.tracks[trackAlias]
	return ok && entry.receiver != nil
}

// subscribe starts a subscription and returns it together with the reader of
// the stream the session opened for it.
func subscribe(t *testing.T, session *Session, conn *testConnection) (*OutgoingSubscribeRequest, *blockingReader) {
	t.Helper()
	request, err := session.Subscribe(context.Background(), [][]byte{[]byte("namespace")}, "track")
	require.NoError(t, err)
	return request, <-conn.openedStreams
}

// The track alias is assigned by the peer in SUBSCRIBE_OK, so a subgroup stream
// can arrive first. Its objects must be delivered once the alias is known.
func TestSubgroupBeforeSubscribeOk(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribe(t, session, conn)

	reader := conn.acceptUniStream(encodeDataStream(t, 17, 3, 5, testObject{0, "hello"}))
	<-reader.drained

	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))

	o := readObject(t, request)
	assert.Equal(t, []byte("hello"), o.Payload)
	assert.Equal(t, uint64(3), o.GroupID)
	assert.Equal(t, uint64(5), o.SubGroupID)
	assert.Equal(t, uint64(0), o.ObjectID)
	assert.Equal(t, ObjectForwardingPreferenceSubgroup, o.ForwardingPreference)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubgroupAfterSubscribeOk(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribe(t, session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	require.Eventually(t, func() bool {
		return hasTrackAlias(session, 17)
	}, time.Second, time.Millisecond)

	conn.acceptUniStream(encodeDataStream(t, 17, 3, 5, testObject{0, "hello"}))

	assert.Equal(t, []byte("hello"), readObject(t, request).Payload)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// The object ID of the first object on a subgroup stream is its delta, and each
// following object is the previous object ID plus its delta plus one.
func TestSubgroupObjectIDDeltas(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribe(t, session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	require.Eventually(t, func() bool {
		return hasTrackAlias(session, 17)
	}, time.Second, time.Millisecond)

	conn.acceptUniStream(encodeDataStream(t, 17, 3, 5,
		testObject{2, "first"},
		testObject{0, "second"},
		testObject{3, "third"},
	))

	for _, want := range []uint64{2, 3, 7} {
		assert.Equal(t, want, readObject(t, request).ObjectID)
	}

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// Datagrams carry a track alias too and can arrive before SUBSCRIBE_OK.
func TestDatagramBeforeSubscribeOk(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribe(t, session, conn)

	conn.sendDatagram(encodeDatagram(17, 3, 4, "hello"))
	require.Eventually(t, func() bool {
		return trackCount(session) == 1
	}, time.Second, time.Millisecond)

	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))

	o := readObject(t, request)
	assert.Equal(t, []byte("hello"), o.Payload)
	assert.Equal(t, uint64(3), o.GroupID)
	assert.Equal(t, uint64(4), o.ObjectID)
	assert.Equal(t, ObjectForwardingPreferenceDatagram, o.ForwardingPreference)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestDatagramAfterSubscribeOk(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribe(t, session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	require.Eventually(t, func() bool {
		return hasTrackAlias(session, 17)
	}, time.Second, time.Millisecond)

	conn.sendDatagram(encodeDatagram(17, 3, 4, "hello"))

	assert.Equal(t, []byte("hello"), readObject(t, request).Payload)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// Objects buffered for an unbound track alias are capped, and the ones that fit
// are delivered in order.
func TestPendingObjectLimit(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	for i := range maxPendingObjects + 5 {
		session.pushObject(17, &Object{Payload: []byte(fmt.Sprintf("object-%d", i))})
	}

	receiver := &testReceiver{}
	require.NoError(t, session.bindTrackAlias(17, receiver))

	require.Len(t, receiver.objects, maxPendingObjects)
	assert.Equal(t, []byte("object-0"), receiver.objects[0].Payload)
	assert.Equal(t, []byte(fmt.Sprintf("object-%d", maxPendingObjects-1)), receiver.objects[maxPendingObjects-1].Payload)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// The number of unbound track aliases buffering objects is capped.
func TestPendingTrackLimit(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	for i := range uint64(maxPendingTracks) + 1 {
		session.pushObject(i, &Object{Payload: []byte("payload")})
	}
	assert.Equal(t, maxPendingTracks, trackCount(session))

	receiver := &testReceiver{}
	require.NoError(t, session.bindTrackAlias(maxPendingTracks, receiver))
	assert.Empty(t, receiver.objects)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// A track alias may only be bound once.
func TestBindDuplicateTrackAlias(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	require.NoError(t, session.bindTrackAlias(17, &testReceiver{}))
	assert.ErrorIs(t, session.bindTrackAlias(17, &testReceiver{}), errDuplicateTrackAlias)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// Closing a request removes its track alias entry.
func TestCloseRequestRemovesTrackAlias(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, requestStream := subscribe(t, session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	require.Eventually(t, func() bool {
		return hasTrackAlias(session, 17)
	}, time.Second, time.Millisecond)

	require.NoError(t, request.Close())
	assert.Equal(t, 0, trackCount(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
