package moqtransport

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type subgroupStream struct {
	stream   ReceiveStream
	receiver objectReceiver
	session  *Session
	stopped  chan struct{}
}

func newSubgroupStream(stream ReceiveStream, receiver objectReceiver, session *Session) *subgroupStream {
	return &subgroupStream{
		stream:   stream,
		receiver: receiver,
		session:  session,
		stopped:  make(chan struct{}),
	}
}

func (d *subgroupStream) stop() {
	select {
	case <-d.stopped:
		return
	default:
		close(d.stopped)
	}
	d.stream.Stop(uint32(StreamResetErrorCodeCancelled))
}

func (d *subgroupStream) isStopped() bool {
	select {
	case <-d.stopped:
		return true
	default:
		return false
	}
}

// read parses objects from the stream until it ends and hands them to the
// receiver. It must be called from a goroutine tracked by the session
// WaitGroup.
func (d *subgroupStream) read(header *wire.SubgroupHeader, parser messageReader) {
	var (
		firstObject  = true
		lastObjectID uint64
		subgroupID   = header.SubgroupID
	)
	for {
		m, err := parser.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) && !d.isStopped() {
				d.session.closeOnError(err)
			}
			return
		}
		o, ok := m.(*wire.SubgroupObject)
		if !ok {
			d.session.closeWithError(&SessionError{
				Code:   uint64(ErrorCodeProtocolViolation),
				Reason: fmt.Sprintf("unexpected message type: %T", m),
			})
			return
		}
		objectID := o.ObjectIDDelta
		if firstObject {
			if header.SubgroupIDMode() == wire.SubgroupIDModeFirstObject {
				subgroupID = objectID
			}
		} else {
			if o.ObjectIDDelta >= math.MaxUint64-lastObjectID {
				d.session.closeWithError(&SessionError{
					Code:   uint64(ErrorCodeProtocolViolation),
					Reason: "object ID out of range",
				})
				return
			}
			objectID = lastObjectID + o.ObjectIDDelta + 1
		}
		firstObject = false
		lastObjectID = objectID

		status := ObjectStatus(o.ObjectStatus)
		if err := validateObjectStatus(status, o.Properties); err != nil {
			d.session.closeWithError(err)
			return
		}
		priority := defaultPublisherPriority
		if !header.DefaultPriority() {
			priority = header.PublisherPriority
		}

		d.session.logger.Debug("received object", "groupID", header.GroupID, "subgroupID", subgroupID, "objectID", objectID, "status", status, "payloadLength", o.PayloadLength)
		object := &Object{
			GroupID:              header.GroupID,
			ObjectID:             objectID,
			ForwardingPreference: ObjectForwardingPreferenceSubgroup,
			SubGroupID:           subgroupID,
			PublisherPriority:    priority,
			Status:               status,
			EndOfGroup:           header.EndOfGroup(),
			FirstObject:          header.FirstObject(),
			Payload:              o.PayloadReader,
			done:                 make(chan struct{}),
		}
		d.receiver.push(object)
		// The object reads from this stream, so the next one can only be
		// parsed once the receiver is done with it.
		select {
		case <-object.done:
		case <-d.session.ctx.Done():
			return
		}
	}
}
