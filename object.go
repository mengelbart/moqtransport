package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type ObjectForwardingPreference int

const (
	ObjectForwardingPreferenceDatagram ObjectForwardingPreference = iota
	ObjectForwardingPreferenceStream
	ObjectForwardingPreferenceStreamGroup
	ObjectForwardingPreferenceStreamTrack
)

func objectForwardingPreferenceFromMessageType(t wire.ObjectMessageType) ObjectForwardingPreference {
	switch t {
	case wire.ObjectDatagramMessageType:
		return ObjectForwardingPreferenceDatagram
	case wire.ObjectStreamMessageType:
		return ObjectForwardingPreferenceStream
	case wire.StreamHeaderTrackMessageType:
		return ObjectForwardingPreferenceStreamTrack
	case wire.StreamHeaderGroupMessageType:
		return ObjectForwardingPreferenceStreamGroup
	}
	panic("invalid object message type")
}

type Object struct {
	GroupID              uint64
	ObjectID             uint64
	PublisherPriority    uint8
	ForwardingPreference ObjectForwardingPreference

	Payload []byte
}
