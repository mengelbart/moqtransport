package wire

import "fmt"

func setBit(x uint64, bit uint, v bool) uint64 {
	if v {
		return x | (1 << bit)
	}
	return x & ^(1 << bit)
}

func getBit(x uint64, bit uint) bool {
	return (x & (1 << bit)) != 0
}

// Bit positions of the flags in the subgroup header type.
const (
	subgroupBitProperties      uint = 0
	subgroupBitSubgroupIDMode  uint = 1 // two bits wide
	subgroupBitEndOfGroup      uint = 3
	subgroupBitDefaultPriority uint = 5
	subgroupBitFirstObject     uint = 6

	subgroupIDModeMask uint64 = 0b0000_0110

	// Subgroup header types take the form 0b0XX1XXXX and fit in a single byte.
	subgroupTypeFormMask uint64 = 0b1001_0000
	subgroupTypeForm     uint64 = 0b0001_0000
	subgroupTypeMax      uint64 = 0xff
)

// Modes of the two bit SUBGROUP_ID_MODE field of the subgroup header type.
const (
	// SubgroupIDModeZero omits the subgroup ID from the header, it is 0.
	SubgroupIDModeZero uint8 = 0
	// SubgroupIDModeFirstObject omits the subgroup ID from the header, it is the
	// object ID of the first object on the stream.
	SubgroupIDModeFirstObject uint8 = 1
	// SubgroupIDModeExplicit carries the subgroup ID in the header.
	SubgroupIDModeExplicit uint8 = 2
	// SubgroupIDModeReserved is reserved for future use and invalid on the wire.
	SubgroupIDModeReserved uint8 = 3
)

type SubgroupHeader struct {
	typ               uint64
	TrackAlias        uint64 `proto:"varint"`
	GroupID           uint64 `proto:"varint"`
	SubgroupID        uint64 `proto:"varint,if=explicitSubgroupID"`
	PublisherPriority uint8  `proto:"byte,if=!DefaultPriority"`
}

func NewSubgroupHeader(trackAlias, groupID, subgroupID uint64, publisherPriority uint8) *SubgroupHeader {
	m := &SubgroupHeader{
		typ:               subgroupTypeForm,
		TrackAlias:        trackAlias,
		GroupID:           groupID,
		SubgroupID:        subgroupID,
		PublisherPriority: publisherPriority,
	}
	m.SetSubgroupIDMode(SubgroupIDModeExplicit)
	return m
}

func (m *SubgroupHeader) Type() ControlMessageType {
	return ControlMessageType(m.typ)
}

func (m *SubgroupHeader) validType() bool {
	if m.typ > subgroupTypeMax {
		return false
	}
	if m.typ&subgroupTypeFormMask != subgroupTypeForm {
		return false
	}
	return m.SubgroupIDMode() != SubgroupIDModeReserved
}

func (m *SubgroupHeader) Properties() bool {
	return getBit(m.typ, subgroupBitProperties)
}

func (m *SubgroupHeader) SetProperties(v bool) {
	m.typ = setBit(m.typ, subgroupBitProperties, v)
}

func (m *SubgroupHeader) explicitSubgroupID() bool {
	return m.SubgroupIDMode() == SubgroupIDModeExplicit
}

func (m *SubgroupHeader) SubgroupIDMode() uint8 {
	return uint8((m.typ & subgroupIDModeMask) >> subgroupBitSubgroupIDMode)
}

func (m *SubgroupHeader) SetSubgroupIDMode(v uint8) {
	switch v {
	case SubgroupIDModeZero, SubgroupIDModeFirstObject, SubgroupIDModeExplicit:
		m.typ = m.typ&^subgroupIDModeMask | uint64(v)<<subgroupBitSubgroupIDMode
	default:
		panic(fmt.Sprintf("invalid subgroup id mode: %d", v))
	}
}

func (m *SubgroupHeader) EndOfGroup() bool {
	return getBit(m.typ, subgroupBitEndOfGroup)
}

func (m *SubgroupHeader) SetEndOfGroup(v bool) {
	m.typ = setBit(m.typ, subgroupBitEndOfGroup, v)
}

func (m *SubgroupHeader) DefaultPriority() bool {
	return getBit(m.typ, subgroupBitDefaultPriority)
}

func (m *SubgroupHeader) SetDefaultPriority(v bool) {
	m.typ = setBit(m.typ, subgroupBitDefaultPriority, v)
}

func (m *SubgroupHeader) FirstBit() bool {
	return getBit(m.typ, subgroupBitFirstObject)
}

func (m *SubgroupHeader) SetFirstBit(v bool) {
	m.typ = setBit(m.typ, subgroupBitFirstObject, v)
}
