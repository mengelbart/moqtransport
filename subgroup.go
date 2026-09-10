package moqtransport

import (
	"errors"
	"fmt"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var (
	errObjectOpen         = errors.New("subgroup already has an open object")
	errObjectWriterClosed = errors.New("object writer is closed")
	errPayloadTooLong     = errors.New("payload longer than the declared object length")
	errPayloadTooShort    = errors.New("payload shorter than the declared object length")
)

type Subgroup struct {
	stream   SendStream
	appender *wire.Appender

	firstObject  bool
	lastObjectID uint64
	open         *ObjectWriter
	err          error
}

func newSubgroup(stream SendStream, version, trackAlias, groupID, subgroupID uint64, publisherPriority uint8) (*Subgroup, error) {
	appender := wire.NewAppender(stream, version)
	if err := appender.Write(wire.NewSubgroupHeader(trackAlias, groupID, subgroupID, publisherPriority)); err != nil {
		return nil, err
	}
	return &Subgroup{
		stream:      stream,
		appender:    appender,
		firstObject: true,
	}, nil
}

// OpenObject starts an object whose payload length is known. The payload goes
// out as it is written to the returned writer, which accepts exactly length
// bytes. Object IDs must increase strictly monotonically along the subgroup,
// and only one object may be open at a time.
func (s *Subgroup) OpenObject(objectID, length uint64) (*ObjectWriter, error) {
	delta, err := s.nextObject(objectID)
	if err != nil {
		return nil, err
	}
	if err := s.appender.WriteObjectHeader(&wire.SubgroupObject{
		ObjectIDDelta: delta,
		PayloadLength: length,
	}); err != nil {
		return nil, err
	}
	s.open = &ObjectWriter{
		subgroup:  s,
		remaining: length,
	}
	return s.open, nil
}

// BufferObject starts an object whose payload length is not known yet. The
// payload is held until Close writes the object. Object IDs must increase
// strictly monotonically along the subgroup, and only one object may be open at
// a time.
func (s *Subgroup) BufferObject(objectID uint64) (*ObjectWriter, error) {
	delta, err := s.nextObject(objectID)
	if err != nil {
		return nil, err
	}
	s.open = &ObjectWriter{
		subgroup: s,
		delta:    delta,
		buffered: true,
	}
	return s.open, nil
}

func (s *Subgroup) nextObject(objectID uint64) (uint64, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.open != nil {
		return 0, errObjectOpen
	}
	delta := objectID
	if !s.firstObject {
		if objectID <= s.lastObjectID {
			return 0, fmt.Errorf("object ID %v not greater than previous object ID %v", objectID, s.lastObjectID)
		}
		delta = objectID - s.lastObjectID - 1
	}
	s.firstObject = false
	s.lastObjectID = objectID
	return delta, nil
}

func (s *Subgroup) fail(err error) error {
	s.err = err
	s.stream.Reset(uint32(StreamResetErrorCodeInternal))
	return err
}

// Close closes the subgroup.
func (s *Subgroup) Close() error {
	// TODO
	return nil
}

// An ObjectWriter writes the payload of one object.
type ObjectWriter struct {
	subgroup  *Subgroup
	remaining uint64
	delta     uint64
	buffer    []byte
	buffered  bool
	closed    bool
}

func (w *ObjectWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errObjectWriterClosed
	}
	if err := w.subgroup.err; err != nil {
		return 0, err
	}
	if w.buffered {
		w.buffer = append(w.buffer, p...)
		return len(p), nil
	}
	if uint64(len(p)) > w.remaining {
		return 0, errPayloadTooLong
	}
	n, err := w.subgroup.stream.Write(p)
	w.remaining -= uint64(n)
	if err != nil {
		return n, w.subgroup.fail(err)
	}
	return n, nil
}

// Close ends the object. A payload shorter than the length the object declared
// leaves the stream unusable, so it is reset.
func (w *ObjectWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	w.subgroup.open = nil
	if err := w.subgroup.err; err != nil {
		return err
	}
	if w.buffered {
		return w.subgroup.appender.Write(&wire.SubgroupObject{
			ObjectIDDelta: w.delta,
			Payload:       w.buffer,
		})
	}
	if w.remaining > 0 {
		return w.subgroup.fail(fmt.Errorf("%w: %v bytes missing", errPayloadTooShort, w.remaining))
	}
	return nil
}
