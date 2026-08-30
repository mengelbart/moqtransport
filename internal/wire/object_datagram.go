package wire

import (
	"bytes"

	"github.com/mengelbart/moqtransport/varint"
)

type ObjectDatagram struct {
	typ               uint64
	TrackAlias        uint64         `proto:"varint"`
	GroupID           uint64         `proto:"varint"`
	ObjectID          uint64         `proto:"varint,if=!ZeroObjectID"`
	PublisherPriority uint8          `proto:"byte,if=!DefaultPriority"`
	Properties        []KeyValuePair `proto:"message_list,if=HasProperties"`
	ObjectStatus      uint64         `proto:"varint,if=Status"`
	ObjectPayload     []byte         `proto:"remaining_bytes,if=!Status"`
}

func (m *ObjectDatagram) Type() ControlMessageType {
	return ControlMessageType(m.typ)
}

func (m *ObjectDatagram) HasProperties() bool {
	return getBit(m.typ, 0)
}

func (m *ObjectDatagram) SetHasProperties(v bool) {
	m.typ = setBit(m.typ, 0, v)
}

func (m *ObjectDatagram) EndOfGroup() bool {
	return getBit(m.typ, 1)
}

func (m *ObjectDatagram) SetEndOfGroup(v bool) {
	m.typ = setBit(m.typ, 1, v)
}

func (m *ObjectDatagram) ZeroObjectID() bool {
	return getBit(m.typ, 2)
}

func (m *ObjectDatagram) SetZeroObjectID(v bool) {
	m.typ = setBit(m.typ, 2, v)
}

func (m *ObjectDatagram) DefaultPriority() bool {
	return getBit(m.typ, 3)
}

func (m *ObjectDatagram) SetDefaultPriority(v bool) {
	m.typ = setBit(m.typ, 3, v)
}

func (m *ObjectDatagram) Status() bool {
	return getBit(m.typ, 5)
}

func (m *ObjectDatagram) SetStatus(v bool) {
	m.typ = setBit(m.typ, 5, v)
}

func (m *ObjectDatagram) AppendDatagram(buf []byte) []byte {
	buf = varint.Append(buf, m.typ)
	return m.append_v18(buf)
}

func (m *ObjectDatagram) Parse(data []byte) error {
	br := bytes.NewReader(data)

	typ, err := varint.Read(br)
	if err != nil {
		return err
	}
	m.typ = typ

	r := &boundedReader{reader: br}
	r.reset(int64(br.Len()))
	return m.parse_v18(r)
}
