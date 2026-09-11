package moqtransport

import (
	"io"
	"sync"
)

type ObjectForwardingPreference int

const (
	ObjectForwardingPreferenceSubgroup ObjectForwardingPreference = iota
	ObjectForwardingPreferenceDatagram
)

// ObjectStatus marks an object as a normal object or as the end of a group or
// track.
type ObjectStatus uint64

const (
	ObjectStatusNormal     ObjectStatus = 0x0
	ObjectStatusEndOfGroup ObjectStatus = 0x3
	ObjectStatusEndOfTrack ObjectStatus = 0x4
)

func (s ObjectStatus) valid() bool {
	switch s {
	case ObjectStatusNormal, ObjectStatusEndOfGroup, ObjectStatusEndOfTrack:
		return true
	}
	return false
}

// An Object is a MoQ Object.
type Object struct {
	GroupID              uint64
	ObjectID             uint64
	ForwardingPreference ObjectForwardingPreference
	SubGroupID           uint64
	PublisherPriority    uint8
	Status               ObjectStatus
	EndOfGroup           bool
	FirstObject          bool
	// Payload reads the object payload. An object received on a data stream
	// reads directly from that stream, so the payload is valid until the next
	// ReadObject on the same subscription, or until Close.
	Payload io.Reader

	closeOnce sync.Once
	done      chan struct{}
}

// Close releases the object. The stream it arrived on stays blocked until then,
// which is what keeps the peer from sending faster than the payloads are read.
func (o *Object) Close() error {
	o.closeOnce.Do(func() {
		if o.done != nil {
			close(o.done)
		}
	})
	return nil
}
