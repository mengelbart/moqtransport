package wire

import (
	"errors"
	"io"

	"github.com/mengelbart/moqtransport/varint"
)

var errNoMessageLength = errors.New("message is not length delimited")

// messageReader reads the body of a single message.
type messageReader interface {
	io.Reader
	io.ByteReader
	remaining() int64
}

// boundedReader reads at most n bytes of a length delimited message body.
// Running out of budget, or reaching the end of the underlying stream mid
// message, is io.ErrUnexpectedEOF, so that io.EOF keeps meaning that a stream
// ended between messages.
type boundedReader struct {
	reader streamReader
	n      int64
}

func (r *boundedReader) reset(n int64) {
	r.n = n
}

func (r *boundedReader) remaining() int64 {
	return r.n
}

func (r *boundedReader) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	if r.n <= 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if int64(len(buf)) > r.n {
		buf = buf[:r.n]
	}
	n, err := r.reader.Read(buf)
	r.n -= int64(n)
	if errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (r *boundedReader) ReadByte() (byte, error) {
	if r.n <= 0 {
		return 0, io.ErrUnexpectedEOF
	}
	b, err := r.reader.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return 0, err
	}
	r.n--
	return b, nil
}

func (r *boundedReader) discard() error {
	var scratch [512]byte
	for r.n > 0 {
		buf := scratch[:]
		if int64(len(buf)) > r.n {
			buf = buf[:r.n]
		}
		n, err := io.ReadFull(r.reader, buf)
		r.n -= int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return io.ErrUnexpectedEOF
			}
			return err
		}
	}
	return nil
}

// unboundedReader reads a message that is not length delimited. The stream
// ending before the message starts is io.EOF, once the message has begun it is
// io.ErrUnexpectedEOF.
type unboundedReader struct {
	reader  streamReader
	started bool
}

func (r *unboundedReader) reset() {
	r.started = false
}

func (r *unboundedReader) remaining() int64 {
	return -1
}

func (r *unboundedReader) Read(buf []byte) (int, error) {
	n, err := r.reader.Read(buf)
	if n > 0 {
		r.started = true
	}
	if errors.Is(err, io.EOF) && r.started {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (r *unboundedReader) ReadByte() (byte, error) {
	b, err := r.reader.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) && r.started {
			err = io.ErrUnexpectedEOF
		}
		return 0, err
	}
	r.started = true
	return b, nil
}

// readBytes reads n bytes. A length delimited message bounds the allocation by
// its remaining budget, otherwise the buffer grows in capped chunks so that a
// bogus length allocates only what actually arrives.
func readBytes(r messageReader, n uint64) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	if rem := r.remaining(); rem >= 0 {
		if n > uint64(rem) {
			return nil, io.ErrUnexpectedEOF
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf, nil
	}

	const chunk = 4096
	buf := make([]byte, 0, min(n, chunk))
	for uint64(len(buf)) < n {
		want := min(n-uint64(len(buf)), chunk)
		buf = append(buf, make([]byte, want)...)
		if _, err := io.ReadFull(r, buf[uint64(len(buf))-want:]); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// tlvReader reads a byte length and returns a reader bounded to it, for a block
// that is length delimited inside a message.
func tlvReader(r messageReader) (*boundedReader, error) {
	n, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	if rem := r.remaining(); rem >= 0 && n > uint64(rem) {
		return nil, io.ErrUnexpectedEOF
	}
	return &boundedReader{reader: r, n: int64(n)}, nil
}

// readRemaining reads the rest of a length delimited message body.
func readRemaining(r messageReader) ([]byte, error) {
	n := r.remaining()
	if n < 0 {
		return nil, errNoMessageLength
	}
	return readBytes(r, uint64(n))
}
