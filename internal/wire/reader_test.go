package wire

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBoundedReader(t *testing.T, data []byte, n int64) *boundedReader {
	t.Helper()
	r := &boundedReader{reader: bufio.NewReader(bytes.NewReader(data))}
	r.reset(n)
	return r
}

func TestBoundedReaderBudget(t *testing.T) {
	r := newTestBoundedReader(t, []byte("abcdef"), 3)

	for _, want := range []byte("abc") {
		b, err := r.ReadByte()
		require.NoError(t, err)
		assert.Equal(t, want, b)
	}
	assert.Equal(t, int64(0), r.remaining())

	_, err := r.ReadByte()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestBoundedReaderReadTruncatesToBudget(t *testing.T) {
	r := newTestBoundedReader(t, []byte("abcdef"), 2)

	buf := make([]byte, 6)
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, []byte("ab"), buf[:n])
	assert.Equal(t, int64(0), r.remaining())
}

func TestBoundedReaderShortStreamIsUnexpectedEOF(t *testing.T) {
	t.Run("ReadByte", func(t *testing.T) {
		r := newTestBoundedReader(t, []byte("ab"), 5)
		for range 2 {
			_, err := r.ReadByte()
			require.NoError(t, err)
		}
		_, err := r.ReadByte()
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("Read", func(t *testing.T) {
		r := newTestBoundedReader(t, []byte("ab"), 5)
		_, err := io.ReadFull(r, make([]byte, 5))
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})
}

func TestBoundedReaderDiscard(t *testing.T) {
	underlying := bufio.NewReader(bytes.NewReader([]byte("abcdef")))
	r := &boundedReader{reader: underlying}
	r.reset(4)

	_, err := r.ReadByte()
	require.NoError(t, err)
	require.NoError(t, r.discard())
	assert.Equal(t, int64(0), r.remaining())

	// The bytes after the message body are still there for the next read.
	rest, err := io.ReadAll(underlying)
	require.NoError(t, err)
	assert.Equal(t, []byte("ef"), rest)
}

func TestBoundedReaderDiscardShortStream(t *testing.T) {
	r := newTestBoundedReader(t, []byte("ab"), 5)
	assert.ErrorIs(t, r.discard(), io.ErrUnexpectedEOF)
}

func TestUnboundedReaderEOFOnlyBeforeTheMessage(t *testing.T) {
	newReader := func(data []byte) *unboundedReader {
		return &unboundedReader{reader: bufio.NewReader(bytes.NewReader(data))}
	}

	assert.Equal(t, int64(-1), newReader(nil).remaining())

	t.Run("nothing read is a clean end", func(t *testing.T) {
		_, err := newReader(nil).ReadByte()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("ending after a byte is unexpected", func(t *testing.T) {
		r := newReader([]byte("a"))
		_, err := r.ReadByte()
		require.NoError(t, err)

		_, err = r.ReadByte()
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("reset starts a new message", func(t *testing.T) {
		r := newReader([]byte("a"))
		_, err := r.ReadByte()
		require.NoError(t, err)

		r.reset()
		_, err = r.ReadByte()
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestReadBytes(t *testing.T) {
	t.Run("bounded", func(t *testing.T) {
		r := newTestBoundedReader(t, []byte("abcdef"), 4)
		buf, err := readBytes(r, 3)
		require.NoError(t, err)
		assert.Equal(t, []byte("abc"), buf)
		assert.Equal(t, int64(1), r.remaining())
	})

	t.Run("longer than the budget", func(t *testing.T) {
		r := newTestBoundedReader(t, []byte("abcdef"), 2)
		_, err := readBytes(r, 3)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("unbounded", func(t *testing.T) {
		r := &unboundedReader{reader: bufio.NewReader(bytes.NewReader([]byte("abcdef")))}
		buf, err := readBytes(r, 3)
		require.NoError(t, err)
		assert.Equal(t, []byte("abc"), buf)
	})

	// A length no stream could deliver must not be allocated up front.
	t.Run("unbounded bogus length", func(t *testing.T) {
		r := &unboundedReader{reader: bufio.NewReader(bytes.NewReader([]byte("abcdef")))}
		_, err := readBytes(r, 1<<60)
		assert.Error(t, err)
	})
}
