package wire

import (
	"io"

	"github.com/mengelbart/moqtransport/varint"
)

type ObjectDatagram struct {
	typ               uint64
	TrackAlias        uint64
	GroupID           uint64
	ObjectID          uint64
	PublisherPriority uint8
	Properties        []KeyValuePair
	ObjectStatus      uint64
	ObjectPayload     []byte
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
	buf = varint.Append(buf, m.TrackAlias)
	buf = varint.Append(buf, m.GroupID)
	if !m.ZeroObjectID() {
		buf = varint.Append(buf, m.ObjectID)
	}
	if !m.DefaultPriority() {
		buf = append(buf, m.PublisherPriority)
	}
	if m.HasProperties() {
		buf = varint.Append(buf, uint64(len(m.Properties)))
		for _, v := range m.Properties {
			buf = varint.Append(buf, uint64(v.Type))
			if v.Type%2 == 0 {
				buf = varint.Append(buf, uint64(v.Varint))
			} else {
				buf = varint.Append(buf, uint64(len(v.Bytes)))
				buf = append(buf, v.Bytes...)
			}
		}
	}
	if m.Status() {
		buf = varint.Append(buf, m.ObjectStatus)
	} else {
		buf = append(buf, m.ObjectPayload...)
	}
	return buf
}

func (m *ObjectDatagram) Parse(data []byte) (parsed int, err error) {
	var n int
	m.typ, n, err = varint.Parse(data)
	parsed += n
	if err != nil {
		return parsed, err
	}
	data = data[n:]

	m.TrackAlias, n, err = varint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]

	m.GroupID, n, err = varint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]

	if !m.ZeroObjectID() {
		m.ObjectID, n, err = varint.Parse(data)
		parsed += n
		if err != nil {
			return
		}
		data = data[n:]
	}

	if !m.DefaultPriority() {
		if len(data) == 0 {
			return parsed, io.ErrUnexpectedEOF
		}
		m.PublisherPriority = data[0]
		parsed += 1
		data = data[1:]
	}

	if m.HasProperties() {
		var numProperties uint64
		numProperties, n, err = varint.Parse(data)
		parsed += n
		if err != nil {
			return
		}
		data = data[n:]

		m.Properties = make([]KeyValuePair, 0)
		for range numProperties {
			var typ uint64
			typ, n, err = varint.Parse(data)
			parsed += n
			if err != nil {
				return
			}
			data = data[n:]

			if typ%2 == 0 {
				var val uint64
				val, n, err = varint.Parse(data)
				parsed += n
				if err != nil {
					return
				}
				data = data[n:]
				m.Properties = append(m.Properties, KeyValuePair{
					Type:   typ,
					Varint: val,
				})
			} else {
				var length uint64
				length, n, err = varint.Parse(data)
				parsed += n
				if err != nil {
					return
				}
				data = data[n:]

				if len(data) < int(length) {
					return parsed, io.ErrUnexpectedEOF
				}
				b := make([]byte, length)
				n = copy(b, data)
				parsed += n
				data = data[n:]
				m.Properties = append(m.Properties, KeyValuePair{
					Type:  typ,
					Bytes: b,
				})
			}
		}
	}

	if m.Status() {
		m.ObjectStatus, n, err = varint.Parse(data)
		parsed += n
		if err != nil {
			return
		}
	} else {
		m.ObjectPayload = make([]byte, len(data))
		n = copy(m.ObjectPayload, data)
		parsed += n
	}
	return
}
