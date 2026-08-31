package wire

import "fmt"

// Bit positions and masks of the fetch object serialization flags.
const (
	fetchBitObjectIDDelta uint = 2
	fetchBitGroupIDDelta  uint = 3
	fetchBitPriority      uint = 4
	fetchBitProperties    uint = 5
	fetchBitDatagram      uint = 6

	fetchSubgroupIDModeMask uint64 = 0b0000_0011

	// Serialization flags below 128 are a bit field.
	fetchFlagsMax uint64 = 0x7f
)

// Encodings of the subgroup ID, the two least significant bits of the
// serialization flags.
const (
	// FetchSubgroupIDModeZero omits the subgroup ID, it is 0.
	FetchSubgroupIDModeZero uint8 = 0
	// FetchSubgroupIDModePrior omits the subgroup ID, it is the prior object's.
	FetchSubgroupIDModePrior uint8 = 1
	// FetchSubgroupIDModePriorPlusOne omits the subgroup ID, it is the prior
	// object's plus one.
	FetchSubgroupIDModePriorPlusOne uint8 = 2
	// FetchSubgroupIDModeExplicit carries the subgroup ID in the object.
	FetchSubgroupIDModeExplicit uint8 = 3
)

// Serialization flag values at or above 128.
const (
	FetchEndOfNonExistentRange uint64 = 0x8c
	FetchEndOfUnknownRange     uint64 = 0x10c
)

var errInvalidSerializationFlags = fmt.Errorf("invalid fetch object serialization flags")

type FetchObject struct {
	flags uint64

	GroupIDDelta      uint64         `proto:"varint,if=HasGroupIDDelta"`
	SubgroupID        uint64         `proto:"varint,if=explicitSubgroupID"`
	ObjectIDDelta     uint64         `proto:"varint,if=HasObjectIDDelta"`
	PublisherPriority uint8          `proto:"byte,if=HasPriority"`
	Properties        []KeyValuePair `proto:"kvp_list_tlv,if=HasProperties"`
	ObjectPayload     []byte         `proto:"tlv_bytes"`
}

func NewEndOfNonExistentRange(groupIDDelta, objectIDDelta uint64) *FetchObject {
	return newEndOfRange(FetchEndOfNonExistentRange, groupIDDelta, objectIDDelta)
}

func NewEndOfUnknownRange(groupIDDelta, objectIDDelta uint64) *FetchObject {
	return newEndOfRange(FetchEndOfUnknownRange, groupIDDelta, objectIDDelta)
}

func newEndOfRange(flags, groupIDDelta, objectIDDelta uint64) *FetchObject {
	return &FetchObject{
		flags:         flags,
		GroupIDDelta:  groupIDDelta,
		ObjectIDDelta: objectIDDelta,
	}
}

func (m *FetchObject) Type() ControlMessageType {
	return ControlMessageType(m.flags)
}

// validate rejects a serialization flags value that is neither a bit field nor
// one of the two end of range markers.
func (m *FetchObject) validate() error {
	if m.flags <= fetchFlagsMax || m.IsEndOfRange() {
		return nil
	}
	return fmt.Errorf("%w: %v", errInvalidSerializationFlags, m.flags)
}

func (m *FetchObject) IsEndOfRange() bool {
	return m.flags == FetchEndOfNonExistentRange || m.flags == FetchEndOfUnknownRange
}

func (m *FetchObject) SubgroupIDMode() uint8 {
	return uint8(m.flags & fetchSubgroupIDModeMask)
}

func (m *FetchObject) SetSubgroupIDMode(v uint8) {
	switch v {
	case FetchSubgroupIDModeZero, FetchSubgroupIDModePrior, FetchSubgroupIDModePriorPlusOne, FetchSubgroupIDModeExplicit:
		m.flags = m.flags&^fetchSubgroupIDModeMask | uint64(v)
	default:
		panic(fmt.Sprintf("invalid fetch subgroup id mode: %d", v))
	}
}

// explicitSubgroupID reports whether the subgroup ID field is present. A
// datagram object has no subgroup ID and the mode bits are ignored.
func (m *FetchObject) explicitSubgroupID() bool {
	return !m.Datagram() && m.SubgroupIDMode() == FetchSubgroupIDModeExplicit
}

func (m *FetchObject) HasObjectIDDelta() bool {
	return getBit(m.flags, fetchBitObjectIDDelta)
}

func (m *FetchObject) SetHasObjectIDDelta(v bool) {
	m.flags = setBit(m.flags, fetchBitObjectIDDelta, v)
}

func (m *FetchObject) HasGroupIDDelta() bool {
	return getBit(m.flags, fetchBitGroupIDDelta)
}

func (m *FetchObject) SetHasGroupIDDelta(v bool) {
	m.flags = setBit(m.flags, fetchBitGroupIDDelta, v)
}

func (m *FetchObject) HasPriority() bool {
	return getBit(m.flags, fetchBitPriority)
}

func (m *FetchObject) SetHasPriority(v bool) {
	m.flags = setBit(m.flags, fetchBitPriority, v)
}

func (m *FetchObject) HasProperties() bool {
	return getBit(m.flags, fetchBitProperties)
}

func (m *FetchObject) SetHasProperties(v bool) {
	m.flags = setBit(m.flags, fetchBitProperties, v)
}

func (m *FetchObject) Datagram() bool {
	return getBit(m.flags, fetchBitDatagram)
}

func (m *FetchObject) SetDatagram(v bool) {
	m.flags = setBit(m.flags, fetchBitDatagram, v)
}
