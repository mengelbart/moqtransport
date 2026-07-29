package wire2

import (
	"io"

	"github.com/mengelbart/moqtransport/varint"
)

type ObjectStreamReader interface {
	io.Reader
	io.ByteReader
}

type ObjectStream struct {
	HasProperties bool

	ObjectIDDelta uint64
	Properties    []KeyValuePair
	ObjectStatus  uint64
	ObjectPayload []byte
}

func (m *ObjectStream) Type() ControlMessageType {
	return 0
}

func (m *ObjectStream) append_v18(buf []byte) []byte {
	buf = varint.Append(buf, m.ObjectIDDelta)
	if m.HasProperties {
		buf = varint.Append(buf, uint64(len(m.Properties)))
		for _, prop := range m.Properties {
			buf = varint.Append(buf, prop.Type)
			if prop.Type%2 == 0 {
				buf = varint.Append(buf, prop.Varint)
				continue
			}
			buf = varint.Append(buf, uint64(len(prop.Bytes)))
			buf = append(buf, prop.Bytes...)
		}
	}

	if len(m.ObjectPayload) == 0 {
		buf = varint.Append(buf, 0)
		buf = varint.Append(buf, m.ObjectStatus)
		return buf
	}
	buf = varint.Append(buf, uint64(len(m.ObjectPayload)))
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func (m *ObjectStream) parse_v18(data []byte) error {
	panic("not implemented")
}

func (m *ObjectStream) parse(r ObjectStreamReader) error {
	var err error
	m.ObjectIDDelta, err = varint.Read(r)
	if err != nil {
		return err
	}

	if m.HasProperties {
		var numProperties uint64
		numProperties, err = varint.Read(r)
		if err != nil {
			return err
		}

		m.Properties = make([]KeyValuePair, numProperties)
		for i := range m.Properties {
			var typ uint64
			typ, err = varint.Read(r)
			if err != nil {
				return err
			}
			m.Properties[i].Type = typ

			if typ%2 == 0 {
				m.Properties[i].Varint, err = varint.Read(r)
				if err != nil {
					return err
				}
				continue
			}

			var length uint64
			length, err = varint.Read(r)
			if err != nil {
				return err
			}
			m.Properties[i].Bytes = make([]byte, length)
			_, err = io.ReadFull(r, m.Properties[i].Bytes)
			if err != nil {
				return err
			}
		}
	}

	length, err := varint.Read(r)
	if err != nil {
		return err
	}

	if length == 0 {
		m.ObjectStatus, err = varint.Read(r)
		if err != nil {
			return err
		}
		return nil
	}

	m.ObjectPayload = make([]byte, length)
	_, err = io.ReadFull(r, m.ObjectPayload)
	if err != nil {
		return err
	}

	return nil
}
