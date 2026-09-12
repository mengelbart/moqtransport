package moqtransport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
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
		require.NoError(t, appender.Write(&wire.SubgroupObject{
			ObjectIDDelta: o.delta,
			Payload:       []byte(o.payload),
		}))
	}
	return buf.Bytes()
}

func encodeSubgroupStream(t *testing.T, header *wire.SubgroupHeader, objects ...*wire.SubgroupObject) []byte {
	t.Helper()
	var buf bytes.Buffer
	appender := wire.NewAppender(&buf, 18)
	require.NoError(t, appender.Write(header))
	for _, o := range objects {
		o.SetHasProperties(header.Properties())
		require.NoError(t, appender.Write(o))
	}
	return buf.Bytes()
}

type subscribeResult struct {
	request *OutgoingSubscribeRequest
	err     error
}

// subscribe starts a subscription in the background and returns the request
// stream so the test can answer it while Subscribe is still blocked.
func subscribe(t *testing.T, ctx context.Context, session *Session, conn *testConnection) (<-chan subscribeResult, *blockingReader) {
	t.Helper()
	result := make(chan subscribeResult, 1)
	go func() {
		request, err := session.Subscribe(ctx, [][]byte{[]byte("namespace")}, "track")
		result <- subscribeResult{request, err}
	}()
	return result, <-conn.openedStreams
}

func awaitSubscribe(t *testing.T, result <-chan subscribeResult) (*OutgoingSubscribeRequest, error) {
	t.Helper()
	select {
	case r := <-result:
		return r.request, r.err
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Subscribe to return")
		return nil, nil
	}
}

func subscribeBoundWithStream(t *testing.T, session *Session, conn *testConnection, trackAlias uint64) (*OutgoingSubscribeRequest, *blockingReader) {
	t.Helper()
	result, requestStream := subscribe(t, context.Background(), session, conn)
	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: trackAlias}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)
	require.True(t, hasTrackAlias(session, trackAlias))
	return request, requestStream
}

func subscribeBound(t *testing.T, session *Session, conn *testConnection, trackAlias uint64) *OutgoingSubscribeRequest {
	t.Helper()
	request, _ := subscribeBoundWithStream(t, session, conn, trackAlias)
	return request
}

func requireProtocolViolation(t *testing.T, session *Session) {
	t.Helper()
	require.Eventually(t, func() bool {
		return sessionCloseError(session) != nil
	}, time.Second, time.Millisecond)
	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeProtocolViolation)})
}

func encodeDatagram(trackAlias, groupID, objectID uint64, payload string) []byte {
	msg := &wire.DatagramObject{
		TrackAlias:    trackAlias,
		GroupID:       groupID,
		ObjectID:      objectID,
		ObjectPayload: []byte(payload),
	}
	return msg.AppendDatagram(nil)
}

// datagramObject is an object as it arrives from a datagram, carrying its own
// payload.
func datagramObject(payload string) *Object {
	return &Object{
		ForwardingPreference: ObjectForwardingPreferenceDatagram,
		Payload:              bytes.NewReader([]byte(payload)),
	}
}

func readPayload(t *testing.T, o *Object) []byte {
	t.Helper()
	payload, err := io.ReadAll(o.Payload)
	require.NoError(t, err)
	return payload
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

func pendingObjects(s *Session, trackAlias uint64) []*Object {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()
	entry, ok := s.tracks[trackAlias]
	if !ok {
		return nil
	}
	return entry.pending
}

func hasTrackAlias(s *Session, trackAlias uint64) bool {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()
	entry, ok := s.tracks[trackAlias]
	return ok && entry.receiver != nil
}

// The track alias is assigned by the peer in SUBSCRIBE_OK, so a subgroup stream
// can arrive first. Its objects must be delivered once the alias is known.
func TestSubgroupBeforeSubscribeOk(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)

	conn.acceptUniStream(encodeDataStream(t, 17, 3, 5, testObject{0, "hello"}))
	// The stream holds the object until the alias is bound, so the entry for it
	// shows that the object arrived first.
	require.Eventually(t, func() bool {
		return trackCount(session) == 1
	}, time.Second, time.Millisecond)

	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)

	o := readObject(t, request)
	assert.Equal(t, []byte("hello"), readPayload(t, o))
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

	request := subscribeBound(t, session, conn, 17)

	conn.acceptUniStream(encodeDataStream(t, 17, 3, 5, testObject{0, "hello"}))

	assert.Equal(t, []byte("hello"), readPayload(t, readObject(t, request)))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// The object ID of the first object on a subgroup stream is its delta, and each
// following object is the previous object ID plus its delta plus one.
func TestSubgroupObjectIDDeltas(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

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

func TestSubgroupObjectMetadata(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

	header := wire.NewSubgroupHeader(17, 3, 5, 42)
	header.SetEndOfGroup(true)
	header.SetFirstObject(true)
	header.SetProperties(true)
	conn.acceptUniStream(encodeSubgroupStream(t, header,
		&wire.SubgroupObject{
			Payload:    []byte("hello"),
			Properties: []wire.KeyValuePair{{Type: 2, Varint: 7}},
		},
		&wire.SubgroupObject{
			ObjectStatus: uint64(ObjectStatusEndOfGroup),
		},
	))

	o := readObject(t, request)
	assert.Equal(t, ObjectStatusNormal, o.Status)
	assert.Equal(t, uint8(42), o.PublisherPriority)
	assert.True(t, o.EndOfGroup)
	assert.True(t, o.FirstObject)
	assert.Equal(t, []byte("hello"), readPayload(t, o))

	o = readObject(t, request)
	assert.Equal(t, uint64(1), o.ObjectID)
	assert.Equal(t, ObjectStatusEndOfGroup, o.Status)
	assert.Empty(t, readPayload(t, o))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubgroupDefaultPriority(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

	header := wire.NewSubgroupHeader(17, 3, 5, 0)
	header.SetDefaultPriority(true)
	conn.acceptUniStream(encodeSubgroupStream(t, header, &wire.SubgroupObject{Payload: []byte("hello")}))

	o := readObject(t, request)
	assert.Equal(t, defaultPublisherPriority, o.PublisherPriority)
	assert.False(t, o.EndOfGroup)
	assert.False(t, o.FirstObject)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubgroupUnknownObjectStatus(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	subscribeBound(t, session, conn, 17)

	conn.acceptUniStream(encodeSubgroupStream(t, wire.NewSubgroupHeader(17, 3, 5, 0),
		&wire.SubgroupObject{ObjectStatus: 0x1},
	))

	requireProtocolViolation(t, session)
	goleak.VerifyNone(t)
}

func TestSubgroupPropertiesOnStatusObject(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	subscribeBound(t, session, conn, 17)

	header := wire.NewSubgroupHeader(17, 3, 5, 0)
	header.SetProperties(true)
	conn.acceptUniStream(encodeSubgroupStream(t, header,
		&wire.SubgroupObject{
			ObjectStatus: uint64(ObjectStatusEndOfTrack),
			Properties:   []wire.KeyValuePair{{Type: 2, Varint: 7}},
		},
	))

	requireProtocolViolation(t, session)
	goleak.VerifyNone(t)
}

func TestDatagramObjectMetadata(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

	msg := &wire.DatagramObject{
		TrackAlias:        17,
		GroupID:           3,
		ObjectID:          4,
		PublisherPriority: 42,
		Properties:        []wire.KeyValuePair{{Type: 2, Varint: 7}},
		ObjectPayload:     []byte("hello"),
	}
	msg.SetEndOfGroup(true)
	msg.SetHasProperties(true)
	conn.sendDatagram(msg.AppendDatagram(nil))

	o := readObject(t, request)
	assert.Equal(t, ObjectStatusNormal, o.Status)
	assert.Equal(t, uint8(42), o.PublisherPriority)
	assert.True(t, o.EndOfGroup)
	assert.False(t, o.FirstObject)
	assert.Equal(t, []byte("hello"), readPayload(t, o))

	msg = &wire.DatagramObject{
		TrackAlias:   17,
		GroupID:      3,
		ObjectID:     5,
		ObjectStatus: uint64(ObjectStatusEndOfTrack),
	}
	msg.SetDefaultPriority(true)
	msg.SetStatus(true)
	conn.sendDatagram(msg.AppendDatagram(nil))

	o = readObject(t, request)
	assert.Equal(t, ObjectStatusEndOfTrack, o.Status)
	assert.Equal(t, defaultPublisherPriority, o.PublisherPriority)
	assert.Empty(t, readPayload(t, o))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestDatagramPropertiesOnStatusObject(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	subscribeBound(t, session, conn, 17)

	msg := &wire.DatagramObject{
		TrackAlias:   17,
		GroupID:      3,
		ObjectID:     4,
		Properties:   []wire.KeyValuePair{{Type: 2, Varint: 7}},
		ObjectStatus: uint64(ObjectStatusEndOfGroup),
	}
	msg.SetHasProperties(true)
	msg.SetStatus(true)
	conn.sendDatagram(msg.AppendDatagram(nil))

	requireProtocolViolation(t, session)
	goleak.VerifyNone(t)
}

// Datagrams carry a track alias too and can arrive before SUBSCRIBE_OK.
func TestDatagramBeforeSubscribeOk(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)

	conn.sendDatagram(encodeDatagram(17, 3, 4, "hello"))
	require.Eventually(t, func() bool {
		return trackCount(session) == 1
	}, time.Second, time.Millisecond)

	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)

	o := readObject(t, request)
	assert.Equal(t, []byte("hello"), readPayload(t, o))
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

	request := subscribeBound(t, session, conn, 17)

	conn.sendDatagram(encodeDatagram(17, 3, 4, "hello"))

	assert.Equal(t, []byte("hello"), readPayload(t, readObject(t, request)))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// Datagram objects buffered for an unbound track alias are capped, and the ones
// that fit are delivered in order.
func TestPendingObjectLimit(t *testing.T) {
	const maxPending = 4

	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithMaxPendingObjects(maxPending))
	require.NoError(t, err)

	for i := range maxPending + 5 {
		session.pushDatagramObject(17, datagramObject(fmt.Sprintf("object-%d", i)))
	}

	receiver := &testReceiver{}
	require.NoError(t, session.bindTrackAlias(17, receiver))

	require.Len(t, receiver.objects, maxPending)
	assert.Equal(t, []byte("object-0"), readPayload(t, receiver.objects[0]))
	assert.Equal(t, []byte(fmt.Sprintf("object-%d", maxPending-1)), readPayload(t, receiver.objects[maxPending-1]))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestPendingTrackLimit(t *testing.T) {
	const maxTracks = 4

	conn := newTestConnection(t)
	session, err := NewSession(conn, "", WithMaxPendingTracks(maxTracks))
	require.NoError(t, err)

	for i := range uint64(maxTracks) {
		session.pushDatagramObject(i, datagramObject("payload"))
	}
	assert.Equal(t, maxTracks, trackCount(session))

	session.pushDatagramObject(maxTracks, datagramObject("payload"))
	assert.Equal(t, maxTracks, trackCount(session))

	<-session.Context().Done()
	var sessionErr *SessionError
	require.ErrorAs(t, context.Cause(session.Context()), &sessionErr)
	assert.Equal(t, uint64(ErrorCodeInternal), sessionErr.Code)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestInvalidLimitOptions(t *testing.T) {
	options := map[string]Option{
		"max pending objects":   WithMaxPendingObjects(0),
		"max pending tracks":    WithMaxPendingTracks(-1),
		"subscribe buffer size": WithSubscribeBufferSize(0),
	}
	for name, option := range options {
		t.Run(name, func(t *testing.T) {
			conn := newTestConnection(t)
			session, err := NewSession(conn, "", option)
			assert.Error(t, err)
			assert.Nil(t, session)
			assert.Equal(t, 0, conn.openedUniStreams())
			goleak.VerifyNone(t)
		})
	}
}

func TestSubgroupStreamWaitsForBind(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	result, requestStream := subscribe(t, context.Background(), session, conn)

	conn.acceptUniStream(encodeDataStream(t, 17, 3, 5,
		testObject{0, "first"},
		testObject{0, "second"},
	))
	require.Eventually(t, func() bool {
		return trackCount(session) == 1
	}, time.Second, time.Millisecond)
	assert.Empty(t, pendingObjects(session, 17))

	requestStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))
	request, err := awaitSubscribe(t, result)
	require.NoError(t, err)

	assert.Equal(t, []byte("first"), readPayload(t, readObject(t, request)))
	assert.Equal(t, []byte("second"), readPayload(t, readObject(t, request)))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubgroupObjectPayloadNotRead(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

	conn.acceptUniStream(encodeDataStream(t, 17, 3, 5,
		testObject{0, "first"},
		testObject{0, "second"},
	))

	assert.Equal(t, uint64(0), readObject(t, request).ObjectID)

	second := readObject(t, request)
	assert.Equal(t, uint64(1), second.ObjectID)
	assert.Equal(t, []byte("second"), readPayload(t, second))

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

func TestDuplicateTrackAliasInSubscribeOkClosesSession(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	subscribeBound(t, session, conn, 17)

	result, secondStream := subscribe(t, context.Background(), session, conn)
	secondStream.feed(encodeControlMessage(t, &wire.SubscribeOk{TrackAlias: 17}))

	_, err = awaitSubscribe(t, result)
	assert.ErrorIs(t, err, &SessionError{Code: uint64(ErrorCodeDuplicateTrackAlias)})
	assert.ErrorIs(t, sessionCloseError(session), &SessionError{Code: uint64(ErrorCodeDuplicateTrackAlias)})

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// Closing a request removes its track alias entry.
func TestCloseRequestRemovesTrackAlias(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request := subscribeBound(t, session, conn, 17)

	require.NoError(t, request.Close())
	assert.Equal(t, 0, trackCount(session))

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func acceptSubscribe(t *testing.T, conn *testConnection, handler *MockHandler) (*IncomingSubscribeRequest, *capturingStream) {
	t.Helper()
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})
	requestID := uint64(0)
	if conn.Perspective() == PerspectiveClient {
		requestID = 1
	}
	_, requestStream := conn.acceptStreamCapturing(encodeControlMessage(t, &wire.Subscribe{
		RequestID:      requestID,
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	return <-requests, requestStream
}

func TestSubgroupCloseFinishesStream(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, _ := acceptSubscribe(t, conn, handler)
	request.Accept(17)

	subgroup, err := request.OpenSubgroup(0, 0, 0)
	require.NoError(t, err)
	writeObject(t, subgroup, 0, "payload")
	assert.Equal(t, 0, conn.closedUniStreams())

	require.NoError(t, subgroup.Close())
	assert.Equal(t, 1, conn.closedUniStreams())
	assert.Empty(t, conn.uniStreamResets())

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestSubgroupResetResetsStream(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, _ := acceptSubscribe(t, conn, handler)
	request.Accept(17)

	subgroup, err := request.OpenSubgroup(0, 0, 0)
	require.NoError(t, err)
	writeObject(t, subgroup, 0, "payload")

	subgroup.Reset(StreamResetErrorCodeCancelled)
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, conn.uniStreamResets())
	assert.Equal(t, 0, conn.closedUniStreams())

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

func TestIncomingSubscribeRequestCloseFinishesStream(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	request, requestStream := acceptSubscribe(t, conn, handler)
	request.Accept(17)

	select {
	case <-requestStream.closed:
		t.Fatal("request stream closed before Close")
	default:
	}
	require.NoError(t, request.Close())
	select {
	case <-requestStream.closed:
	case <-time.After(time.Second):
		t.Fatal("request stream not closed")
	}

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}
