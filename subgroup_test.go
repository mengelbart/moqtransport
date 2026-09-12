package moqtransport

import (
	"bytes"
	"io"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSendStream is the sending end of a stream, keeping what was written to it
// and the codes it was reset with.
type testSendStream struct {
	bytes.Buffer
	resets []uint32
	closed bool
}

func (s *testSendStream) Reset(code uint32) {
	s.resets = append(s.resets, code)
}

func (s *testSendStream) Close() error {
	s.closed = true
	return nil
}

func (s *testSendStream) StreamID() uint64 {
	return 0
}

func newTestSubgroup(t *testing.T) (*Subgroup, *testSendStream) {
	t.Helper()
	stream := &testSendStream{}
	subgroup, err := newSubgroup(stream, 18, 4, 7, 9, 200, nil)
	require.NoError(t, err)
	return subgroup, stream
}

// writeObject writes a complete object with a known payload length.
func writeObject(t *testing.T, s *Subgroup, objectID uint64, payload string) {
	t.Helper()
	object, err := s.OpenObject(objectID, uint64(len(payload)))
	require.NoError(t, err)
	n, err := object.Write([]byte(payload))
	require.NoError(t, err)
	assert.Equal(t, len(payload), n)
	require.NoError(t, object.Close())
}

// readObjects parses the subgroup header and the objects that follow it.
func readObjects(t *testing.T, stream *testSendStream, count int) (*wire.SubgroupHeader, []*wire.SubgroupObject) {
	t.Helper()
	parser, err := wire.NewParser(stream, 18, wire.StreamTypeData)
	require.NoError(t, err)

	msg, err := parser.Read()
	require.NoError(t, err)
	header, ok := msg.(*wire.SubgroupHeader)
	require.True(t, ok)

	objects := make([]*wire.SubgroupObject, 0, count)
	for range count {
		msg, err := parser.Read()
		require.NoError(t, err)
		o, ok := msg.(*wire.SubgroupObject)
		require.True(t, ok)
		payload, err := io.ReadAll(o.PayloadReader)
		require.NoError(t, err)
		o.Payload = payload
		objects = append(objects, o)
	}
	return header, objects
}

func TestSubgroupOpenObject(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	for _, objectID := range []uint64{0, 1, 5} {
		writeObject(t, subgroup, objectID, "payload")
	}

	header, objects := readObjects(t, stream, 3)
	assert.Equal(t, uint64(4), header.TrackAlias)
	assert.Equal(t, uint64(7), header.GroupID)
	assert.Equal(t, uint64(9), header.SubgroupID)
	assert.Equal(t, uint8(200), header.PublisherPriority)

	for i, want := range []uint64{0, 0, 3} {
		assert.Equal(t, want, objects[i].ObjectIDDelta)
		assert.Equal(t, []byte("payload"), objects[i].Payload)
	}
}

func TestSubgroupOpenObjectOutOfOrder(t *testing.T) {
	subgroup, _ := newTestSubgroup(t)

	writeObject(t, subgroup, 3, "payload")

	_, err := subgroup.OpenObject(3, 7)
	assert.Error(t, err)

	_, err = subgroup.OpenObject(2, 7)
	assert.Error(t, err)
}

func TestSubgroupObjectPayloadIsStreamed(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	object, err := subgroup.OpenObject(0, 10)
	require.NoError(t, err)

	written := stream.Len()
	_, err = object.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, written+len("hello"), stream.Len())

	_, err = object.Write([]byte("world"))
	require.NoError(t, err)
	require.NoError(t, object.Close())

	_, objects := readObjects(t, stream, 1)
	assert.Equal(t, []byte("helloworld"), objects[0].Payload)
}

func TestSubgroupObjectPayloadTooLong(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	object, err := subgroup.OpenObject(0, 5)
	require.NoError(t, err)

	_, err = object.Write([]byte("hello world"))
	assert.ErrorIs(t, err, errPayloadTooLong)

	// The object still takes the bytes it declared.
	_, err = object.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, object.Close())

	_, objects := readObjects(t, stream, 1)
	assert.Equal(t, []byte("hello"), objects[0].Payload)
}

func TestSubgroupObjectPayloadTooShort(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	object, err := subgroup.OpenObject(0, 10)
	require.NoError(t, err)
	_, err = object.Write([]byte("hello"))
	require.NoError(t, err)

	assert.ErrorIs(t, object.Close(), errPayloadTooShort)
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeInternal)}, stream.resets)

	_, err = subgroup.OpenObject(1, 1)
	assert.ErrorIs(t, err, errPayloadTooShort)
}

func TestSubgroupBufferObject(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	written := stream.Len()
	object, err := subgroup.BufferObject(0)
	require.NoError(t, err)
	_, err = object.Write([]byte("hello"))
	require.NoError(t, err)
	_, err = object.Write([]byte("world"))
	require.NoError(t, err)
	assert.Equal(t, written, stream.Len())

	require.NoError(t, object.Close())
	assert.Greater(t, stream.Len(), written)

	_, objects := readObjects(t, stream, 1)
	assert.Equal(t, uint64(0), objects[0].ObjectIDDelta)
	assert.Equal(t, []byte("helloworld"), objects[0].Payload)
}

func TestSubgroupOneObjectAtATime(t *testing.T) {
	subgroup, _ := newTestSubgroup(t)

	object, err := subgroup.OpenObject(0, 5)
	require.NoError(t, err)

	_, err = subgroup.OpenObject(1, 5)
	assert.ErrorIs(t, err, errObjectOpen)
	_, err = subgroup.BufferObject(1)
	assert.ErrorIs(t, err, errObjectOpen)

	_, err = object.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, object.Close())

	_, err = subgroup.OpenObject(1, 5)
	assert.NoError(t, err)
}

func TestSubgroupObjectWriterAfterClose(t *testing.T) {
	subgroup, _ := newTestSubgroup(t)

	object, err := subgroup.BufferObject(0)
	require.NoError(t, err)
	require.NoError(t, object.Close())
	// Close is idempotent, writing after it is not allowed.
	require.NoError(t, object.Close())

	_, err = object.Write([]byte("hello"))
	assert.ErrorIs(t, err, errObjectWriterClosed)
}

func TestSubgroupClose(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)
	writeObject(t, subgroup, 0, "payload")

	require.NoError(t, subgroup.Close())
	assert.True(t, stream.closed)
	assert.Empty(t, stream.resets)

	// Close is idempotent, writing after it is not allowed.
	require.NoError(t, subgroup.Close())
	_, err := subgroup.OpenObject(1, 1)
	assert.ErrorIs(t, err, errSubgroupClosed)

	_, objects := readObjects(t, stream, 1)
	assert.Equal(t, []byte("payload"), objects[0].Payload)
}

func TestSubgroupCloseWithOpenObject(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	object, err := subgroup.OpenObject(0, 5)
	require.NoError(t, err)
	assert.ErrorIs(t, subgroup.Close(), errObjectOpen)
	assert.False(t, stream.closed)

	_, err = object.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, object.Close())
	require.NoError(t, subgroup.Close())
	assert.True(t, stream.closed)
}

func TestSubgroupCloseAfterFailure(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)

	object, err := subgroup.OpenObject(0, 10)
	require.NoError(t, err)
	assert.ErrorIs(t, object.Close(), errPayloadTooShort)

	assert.ErrorIs(t, subgroup.Close(), errPayloadTooShort)
	assert.False(t, stream.closed)
}

func TestSubgroupReset(t *testing.T) {
	subgroup, stream := newTestSubgroup(t)
	writeObject(t, subgroup, 0, "payload")

	object, err := subgroup.OpenObject(1, 5)
	require.NoError(t, err)

	subgroup.Reset(StreamResetErrorCodeCancelled)
	assert.Equal(t, []uint32{uint32(StreamResetErrorCodeCancelled)}, stream.resets)
	assert.False(t, stream.closed)

	_, err = object.Write([]byte("hello"))
	assert.ErrorIs(t, err, errSubgroupReset)
	assert.ErrorIs(t, object.Close(), errSubgroupReset)
	_, err = subgroup.OpenObject(2, 1)
	assert.ErrorIs(t, err, errSubgroupReset)
	assert.ErrorIs(t, subgroup.Close(), errSubgroupReset)

	// A second reset does nothing.
	subgroup.Reset(StreamResetErrorCodeInternal)
	assert.Len(t, stream.resets, 1)
}

func TestSubgroupEmptyObject(t *testing.T) {
	streamed, streamedStream := newTestSubgroup(t)
	object, err := streamed.OpenObject(0, 0)
	require.NoError(t, err)
	require.NoError(t, object.Close())

	buffered, bufferedStream := newTestSubgroup(t)
	object, err = buffered.BufferObject(0)
	require.NoError(t, err)
	require.NoError(t, object.Close())

	assert.Equal(t, bufferedStream.Bytes(), streamedStream.Bytes())

	_, objects := readObjects(t, streamedStream, 1)
	assert.Empty(t, objects[0].Payload)
	assert.Equal(t, uint64(0), objects[0].ObjectStatus)
}
