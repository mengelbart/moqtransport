package wire

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/mengelbart/moqtransport/varint"
)

// Datagram types take the form 0b00X0XXXX, bit 4 and everything above bit 5
// must be zero.
const datagramTypeFormMask uint64 = ^uint64(0b0010_1111)

var errEmptyDatagramProperties = errors.New("datagram has PROPERTIES bit set but no properties")

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
	if !m.validType() {
		return fmt.Errorf("invalid datagram type: %d", typ)
	}

	r := &boundedReader{reader: br}
	r.reset(int64(br.Len()))
	if err := m.parse_v18(r); err != nil {
		return err
	}
	if m.HasProperties() && len(m.Properties) == 0 {
		return errEmptyDatagramProperties
	}
	return nil
}

// validType reports whether the type has the form 0b00X0XXXX and does not
// combine the STATUS and END_OF_GROUP bits.
func (m *DatagramObject) validType() bool {
	if m.typ&datagramTypeFormMask != 0 {
		return false
	}
	return !m.Status() || !m.EndOfGroup()
}
