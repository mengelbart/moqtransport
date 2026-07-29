package wire2

import (
	"fmt"

	"github.com/mengelbart/moqtransport/varint"
)

func setBit(x uint64, bit uint, v bool) uint64 {
	if v {
		return x | (1 << bit)
	}
	return x & ^(1 << bit)
}

func getBit(x uint64, bit uint) bool {
	return (x & (1 << bit)) != 0
}

type SubgroupHeader struct {
	typ               uint64
	TrackAlias        uint64
	GroupID           uint64
	SubgroupID        uint64
	PublisherPriority uint8
}

func NewSubgroupHeader() *SubgroupHeader {
	return &SubgroupHeader{
		typ: 0b00010000,
	}
}

func (m *SubgroupHeader) Type() ControlMessageType {
	m.typ = setBit(m.typ, 4, true)
	return ControlMessageType(m.typ)
}

func (m *SubgroupHeader) validType() bool {
	return m.typ&0b10010000 == 0b00010000
}

func (m *SubgroupHeader) Properties() bool {
	return getBit(m.typ, 0)
}

func (m *SubgroupHeader) SetProperties(v bool) {
	m.typ = setBit(m.typ, 0, v)
}

func (m *SubgroupHeader) SubgroupIDMode() uint8 {
	return uint8((m.typ & 0x06) >> 1)
}

func (m *SubgroupHeader) SetSubgroupIDMode(v uint8) {
	switch v {
	case 0:
		m.typ = setBit(m.typ, 1, false)
		m.typ = setBit(m.typ, 2, false)
	case 1:
		m.typ = setBit(m.typ, 1, true)
		m.typ = setBit(m.typ, 2, false)
	case 2:
		m.typ = setBit(m.typ, 1, false)
		m.typ = setBit(m.typ, 2, true)
	default:
		panic(fmt.Sprintf("invalid subgroup id mode: %d", v))
	}
}

func (m *SubgroupHeader) EndOfGroup() bool {
	return getBit(m.typ, 3)
}

func (m *SubgroupHeader) SetEndOfGroup(v bool) {
	m.typ = setBit(m.typ, 3, v)
}

func (m *SubgroupHeader) DefaultPriority() bool {
	return getBit(m.typ, 5)
}

func (m *SubgroupHeader) SetDefaultPriority(v bool) {
	m.typ = setBit(m.typ, 5, v)
}

func (m *SubgroupHeader) FirstBit() bool {
	return getBit(m.typ, 6)
}

func (m *SubgroupHeader) SetFirstBit(v bool) {
	m.typ = setBit(m.typ, 6, v)
}

func (m *SubgroupHeader) append_v18(buf []byte) []byte {
	buf = varint.Append(buf, uint64(m.TrackAlias))
	buf = varint.Append(buf, uint64(m.GroupID))
	switch m.SubgroupIDMode() {
	case 0x00, 0x01: // no subgroup id
		// nothing to do
	case 0x02: // subgroup id present
		buf = varint.Append(buf, uint64(m.SubgroupID))
	default:
		panic("invalid subgroup header type")
	}
	if !m.DefaultPriority() {
		buf = varint.Append(buf, uint64(m.PublisherPriority))
	}
	return buf
}

func (m *SubgroupHeader) parse_v18(data []byte) error {
	var err error
	var n int

	m.TrackAlias, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.GroupID, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if m.typ&0x06>>1 == 0x02 {
		m.SubgroupID, n, err = varint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}

	if m.typ&0x20 == 0 {
		var priority uint64
		priority, n, err = varint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]
		if priority > 255 {
			return fmt.Errorf("publisher priority out of range: %d", priority)
		}
		m.PublisherPriority = uint8(priority)
	}

	return nil
}
