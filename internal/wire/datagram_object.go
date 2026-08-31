package wire

import (
	"bytes"

	"github.com/mengelbart/moqtransport/varint"
)

type DatagramObject struct {
	typ               uint64
	TrackAlias        uint64         `proto:"varint"`
	GroupID           uint64         `proto:"varint"`
	ObjectID          uint64         `proto:"varint,if=!ZeroObjectID"`
	PublisherPriority uint8          `proto:"byte,if=!DefaultPriority"`
	Properties        []KeyValuePair `proto:"kvp_list_tlv,if=HasProperties"`
	ObjectStatus      uint64         `proto:"varint,if=Status"`
	ObjectPayload     []byte         `proto:"remaining_bytes,if=!Status"`
}

func (m *DatagramObject) Type() ControlMessageType {
	return ControlMessageType(m.typ)
}

func (m *DatagramObject) HasProperties() bool {
	return getBit(m.typ, 0)
}

func (m *DatagramObject) SetHasProperties(v bool) {
	m.typ = setBit(m.typ, 0, v)
}

func (m *DatagramObject) EndOfGroup() bool {
	return getBit(m.typ, 1)
}

func (m *DatagramObject) SetEndOfGroup(v bool) {
	m.typ = setBit(m.typ, 1, v)
}

func (m *DatagramObject) ZeroObjectID() bool {
	return getBit(m.typ, 2)
}

func (m *DatagramObject) SetZeroObjectID(v bool) {
	m.typ = setBit(m.typ, 2, v)
}

func (m *DatagramObject) DefaultPriority() bool {
	return getBit(m.typ, 3)
}

func (m *DatagramObject) SetDefaultPriority(v bool) {
	m.typ = setBit(m.typ, 3, v)
}

func (m *DatagramObject) Status() bool {
	return getBit(m.typ, 5)
}

func (m *DatagramObject) SetStatus(v bool) {
	m.typ = setBit(m.typ, 5, v)
}

func (m *DatagramObject) AppendDatagram(buf []byte) []byte {
	buf = varint.Append(buf, m.typ)
	return m.append_v18(buf)
}

func (m *DatagramObject) Parse(data []byte) error {
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
