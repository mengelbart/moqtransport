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
	errTestOptionFailed     = errors.New("test option failed")
	errTestSetupWriteFailed = errors.New("test setup write failed")
)

// blockingReader serves data and then blocks until more is fed or it is closed,
// like a stream that has delivered a message and is waiting for the next one.
type blockingReader struct {
	mu     sync.Mutex
	data   []byte
	notify chan struct{}

	drainedOnce sync.Once
	drained     chan struct{}

	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
}

func newBlockingReader(data []byte) *blockingReader {
	return &blockingReader{
		data:    append([]byte(nil), data...),
		notify:  make(chan struct{}, 1),
		drained: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	for {
		r.mu.Lock()
		if len(r.data) > 0 {
			n := copy(p, r.data)
			r.data = r.data[n:]
			r.mu.Unlock()
			return n, nil
		}
		r.mu.Unlock()

		r.drainedOnce.Do(func() { close(r.drained) })
		select {
		case <-r.closed:
			return 0, r.closeErr
		case <-r.notify:
		}
	}
}

// feed appends data to the reader and wakes a blocked Read.
func (r *blockingReader) feed(data []byte) {
	r.mu.Lock()
	r.data = append(r.data, data...)
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
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
	datagrams   chan []byte

	// openedStreams carries the readers of the bidirectional streams the
	// session opened, so a test can feed responses to its own requests.
	openedStreams chan *blockingReader

	// sendStreamWriteErr, when set, is returned by every write on a stream
	// opened by the session, e.g. to fail the SETUP message. It must be set
	// before the session is created and is read-only afterwards.
	sendStreamWriteErr error

	mu             sync.Mutex
	readers        []*blockingReader
	lastID         uint64
	openedUniCount int
	closeCount     int
}

func newTestConnection(t *testing.T) *testConnection {
	t.Helper()
	ctrl := gomock.NewController(t)
	c := &testConnection{
		MockConnection: NewMockConnection(ctrl),
		ctrl:           ctrl,
		uniStreams:     make(chan ReceiveStream, 1),
		bidiStreams:    make(chan Stream, 1),
		datagrams:      make(chan []byte, 1),
		openedStreams:  make(chan *blockingReader, 8),
	}
	c.EXPECT().ApplicationProtocol().Return(MOQT18).AnyTimes()
	c.EXPECT().Perspective().Return(PerspectiveServer).AnyTimes()
	c.EXPECT().Protocol().Return(ProtocolQUIC).AnyTimes()
	c.EXPECT().OpenUniStream().DoAndReturn(func() (SendStream, error) {
		return c.newSendStream(), nil
	}).AnyTimes()
	c.EXPECT().OpenStreamSync(gomock.Any()).DoAndReturn(func(context.Context) (Stream, error) {
		stream, reader := c.newStream(nil)
		c.openedStreams <- reader
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
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case dgram := <-c.datagrams:
			return dgram, nil
		}
	}).AnyTimes()
	c.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).DoAndReturn(func(uint64, string) error {
		c.mu.Lock()
		c.closeCount++
		readers := c.readers
		c.mu.Unlock()
		for _, r := range readers {
			r.close(errTestConnectionClosed)
		}
		return nil
	}).AnyTimes()
	return c
}

func (c *testConnection) openedUniStreams() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.openedUniCount
}

func (c *testConnection) closes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeCount
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
	c.openedUniCount++
	c.mu.Unlock()

	stream := NewMockSendStream(c.ctrl)
	stream.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		if c.sendStreamWriteErr != nil {
			return 0, c.sendStreamWriteErr
		}
		return len(p), nil
	}).AnyTimes()
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

// sendDatagram delivers a datagram as if the peer had sent it.
func (c *testConnection) sendDatagram(data []byte) {
	c.datagrams <- data
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

// A failing option must be reported before any stream is opened, so there is
// nothing to clean up and the caller's connection is left alone.
func TestNewSessionOptionErrorOpensNoStream(t *testing.T) {
	conn := newTestConnection(t)

	session, err := NewSession(conn, "", func(*Session) error { return errTestOptionFailed })
	assert.ErrorIs(t, err, errTestOptionFailed)
	assert.Nil(t, session)
	assert.Equal(t, 0, conn.openedUniStreams())
	assert.Equal(t, 0, conn.closes())
	goleak.VerifyNone(t)
}

// A failing SETUP write must close the connection, which also tears down the
// control stream that was just opened.
func TestNewSessionSetupWriteErrorClosesConnection(t *testing.T) {
	conn := newTestConnection(t)
	conn.sendStreamWriteErr = errTestSetupWriteFailed

	session, err := NewSession(conn, "")
	assert.ErrorIs(t, err, errTestSetupWriteFailed)
	assert.Nil(t, session)
	assert.Equal(t, 1, conn.openedUniStreams())
	assert.Equal(t, 1, conn.closes())
	goleak.VerifyNone(t)
}
