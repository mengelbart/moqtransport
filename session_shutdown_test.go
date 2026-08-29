package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

var (
	errTestConnectionClosed = errors.New("test connection closed")
	errTestStreamStopped    = errors.New("test stream stopped")
)

// blockingReader serves data and then blocks until it is closed, like a stream
// that has delivered a message and is waiting for the next one.
type blockingReader struct {
	data *bytes.Reader

	drainedOnce sync.Once
	drained     chan struct{}

	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
}

func newBlockingReader(data []byte) *blockingReader {
	return &blockingReader{
		data:    bytes.NewReader(data),
		drained: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if r.data.Len() > 0 {
		return r.data.Read(p)
	}
	r.drainedOnce.Do(func() { close(r.drained) })
	<-r.closed
	return 0, r.closeErr
}

func (r *blockingReader) close(err error) {
	r.closeOnce.Do(func() {
		r.closeErr = err
		close(r.closed)
	})
}

// testConnection drives a MockConnection: it hands out the streams queued on it
// and blocks everywhere else until the connection is closed.
type testConnection struct {
	*MockConnection

	ctrl        *gomock.Controller
	uniStreams  chan ReceiveStream
	bidiStreams chan Stream

	mu      sync.Mutex
	readers []*blockingReader
	lastID  uint64
}

func newTestConnection(t *testing.T) *testConnection {
	t.Helper()
	ctrl := gomock.NewController(t)
	c := &testConnection{
		MockConnection: NewMockConnection(ctrl),
		ctrl:           ctrl,
		uniStreams:     make(chan ReceiveStream, 1),
		bidiStreams:    make(chan Stream, 1),
	}
	c.EXPECT().ApplicationProtocol().Return(MOQT18).AnyTimes()
	c.EXPECT().Perspective().Return(PerspectiveServer).AnyTimes()
	c.EXPECT().Protocol().Return(ProtocolQUIC).AnyTimes()
	c.EXPECT().OpenUniStream().DoAndReturn(func() (SendStream, error) {
		return c.newSendStream(), nil
	}).AnyTimes()
	c.EXPECT().OpenStreamSync(gomock.Any()).DoAndReturn(func(context.Context) (Stream, error) {
		stream, _ := c.newStream(nil)
		return stream, nil
	}).AnyTimes()
	c.EXPECT().AcceptUniStream(gomock.Any()).DoAndReturn(func(ctx context.Context) (ReceiveStream, error) {
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case stream := <-c.uniStreams:
			return stream, nil
		}
	}).AnyTimes()
	c.EXPECT().AcceptStream(gomock.Any()).DoAndReturn(func(ctx context.Context) (Stream, error) {
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case stream := <-c.bidiStreams:
			return stream, nil
		}
	}).AnyTimes()
	c.EXPECT().ReceiveDatagram(gomock.Any()).DoAndReturn(func(ctx context.Context) ([]byte, error) {
		<-ctx.Done()
		return nil, context.Cause(ctx)
	}).AnyTimes()
	c.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).DoAndReturn(func(uint64, string) error {
		c.mu.Lock()
		readers := c.readers
		c.mu.Unlock()
		for _, r := range readers {
			r.close(errTestConnectionClosed)
		}
		return nil
	}).AnyTimes()
	return c
}

func (c *testConnection) newReader(data []byte) (*blockingReader, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := newBlockingReader(data)
	c.readers = append(c.readers, r)
	c.lastID += 4
	return r, c.lastID
}

func (c *testConnection) newSendStream() *MockSendStream {
	c.mu.Lock()
	c.lastID += 4
	id := c.lastID
	c.mu.Unlock()

	stream := NewMockSendStream(c.ctrl)
	stream.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) { return len(p), nil }).AnyTimes()
	stream.EXPECT().Close().Return(nil).AnyTimes()
	stream.EXPECT().Reset(gomock.Any()).AnyTimes()
	stream.EXPECT().StreamID().Return(id).AnyTimes()
	return stream
}

func (c *testConnection) newReceiveStream(data []byte) (*MockReceiveStream, *blockingReader) {
	r, id := c.newReader(data)
	stream := NewMockReceiveStream(c.ctrl)
	stream.EXPECT().Read(gomock.Any()).DoAndReturn(r.Read).AnyTimes()
	stream.EXPECT().Stop(gomock.Any()).Do(func(uint32) { r.close(errTestStreamStopped) }).AnyTimes()
	stream.EXPECT().StreamID().Return(id).AnyTimes()
	return stream, r
}

func (c *testConnection) newStream(data []byte) (*MockStream, *blockingReader) {
	r, id := c.newReader(data)
	stream := NewMockStream(c.ctrl)
	stream.EXPECT().Read(gomock.Any()).DoAndReturn(r.Read).AnyTimes()
	stream.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) { return len(p), nil }).AnyTimes()
	stream.EXPECT().Close().Return(nil).AnyTimes()
	stream.EXPECT().Stop(gomock.Any()).Do(func(uint32) { r.close(errTestStreamStopped) }).AnyTimes()
	stream.EXPECT().Reset(gomock.Any()).AnyTimes()
	stream.EXPECT().StreamID().Return(id).AnyTimes()
	return stream, r
}

// acceptUniStream queues a unidirectional stream carrying data as if the peer
// had opened it.
func (c *testConnection) acceptUniStream(data []byte) *blockingReader {
	stream, r := c.newReceiveStream(data)
	c.uniStreams <- stream
	return r
}

// acceptStream queues a bidirectional stream carrying data as if the peer had
// opened it.
func (c *testConnection) acceptStream(data []byte) *blockingReader {
	stream, r := c.newStream(data)
	c.bidiStreams <- stream
	return r
}

func encodeControlMessage(t *testing.T, msg wire.ControlMessage) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, wire.NewAppender(&buf, 18).Write(msg))
	return buf.Bytes()
}

// The remote control stream reader must not panic when the session is closed,
// and CloseWithError must not return before it is done.
func TestCloseSessionWithRemoteControlStreamReader(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	reader := conn.acceptUniStream(encodeControlMessage(t, &wire.Setup{}))
	<-reader.drained

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// The outgoing subscribe request reader must not panic when the session is
// closed, and CloseWithError must not return before it is done.
func TestCloseSessionWithOutgoingSubscribeRequestReader(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	request, err := session.Subscribe(context.Background(), [][]byte{[]byte("namespace")}, "track")
	require.NoError(t, err)
	require.NotNil(t, request)

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// The incoming subscribe request reader must not panic when the session is
// closed, and CloseWithError must not return before it is done.
func TestCloseSessionWithIncomingSubscribeRequestReader(t *testing.T) {
	conn := newTestConnection(t)
	handler := NewMockHandler(conn.ctrl)
	requests := make(chan *IncomingSubscribeRequest, 1)
	handler.EXPECT().HandleSubscribe(gomock.Any()).Do(func(r *IncomingSubscribeRequest) {
		requests <- r
	})

	session, err := NewSession(conn, "", WithHandler(handler))
	require.NoError(t, err)

	reader := conn.acceptStream(encodeControlMessage(t, &wire.Subscribe{
		TrackNamespace: [][]byte{[]byte("namespace")},
		TrackName:      []byte("track"),
	}))
	assert.Equal(t, []byte("track"), (<-requests).Name())
	<-reader.drained

	session.CloseWithError(0, "closing")
	goleak.VerifyNone(t)
}

// Subscribing on a closed session must not start an untracked reader.
func TestSubscribeAfterCloseFails(t *testing.T) {
	conn := newTestConnection(t)
	session, err := NewSession(conn, "")
	require.NoError(t, err)

	session.CloseWithError(0, "closing")

	_, err = session.Subscribe(context.Background(), [][]byte{[]byte("namespace")}, "track")
	assert.Error(t, err)
	goleak.VerifyNone(t)
}
