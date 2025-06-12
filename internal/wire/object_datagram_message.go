package wire

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	objectTypeDatagram                uint64 = 0x00
	objectTypeDatagramExtension       uint64 = 0x01
	objectTypeDatagramStatus          uint64 = 0x02
	objectTypeDatagramStatusExtension uint64 = 0x03
)

type ObjectDatagramMessage struct {
	TrackAlias             uint64
	GroupID                uint64
	ObjectID               uint64
	PublisherPriority      uint8
	ObjectExtensionHeaders KVPList
	ObjectPayload          []byte
	ObjectStatus           ObjectStatus
}

func (m *ObjectDatagramMessage) AppendDatagram(buf []byte) []byte {
	typ := objectTypeDatagram
	if m.ObjectExtensionHeaders != nil {
		typ = objectTypeDatagramExtension
	}
	buf = quicvarint.Append(buf, typ)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = append(buf, m.PublisherPriority)
	if typ == objectTypeDatagramExtension {
		buf = m.ObjectExtensionHeaders.appendLength(buf)
	}
	return append(buf, m.ObjectPayload...)
}

func (m *ObjectDatagramMessage) AppendDatagramStatus(buf []byte) []byte {
	typ := objectTypeDatagramStatus
	if m.ObjectExtensionHeaders != nil {
		typ = objectTypeDatagramStatusExtension
	}
	buf = quicvarint.Append(buf, typ)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = append(buf, m.PublisherPriority)
	if typ == objectTypeDatagramExtension {
		buf = m.ObjectExtensionHeaders.appendLength(buf)
	}
	return quicvarint.Append(buf, uint64(m.ObjectStatus))
}

func (m *ObjectDatagramMessage) Parse(data []byte) (parsed int, err error) {
	var n int
	var typ uint64
	typ, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return parsed, err
	}
	data = data[n:]

	m.TrackAlias, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]

	m.GroupID, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]

	m.ObjectID, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]

	if len(data) == 0 {
		return parsed, io.ErrUnexpectedEOF
	}
	m.PublisherPriority = data[0]
	parsed += 1
	data = data[1:]

	if typ&0x01 == 1 {
		m.ObjectExtensionHeaders = KVPList{}
		n, err = m.ObjectExtensionHeaders.parseLength(data)
		parsed += n
		if err != nil {
			return parsed, err
		}
	}
	if typ&0x02 == 0 {
		m.ObjectPayload = make([]byte, len(data))
		n = copy(m.ObjectPayload, data)
		parsed += n
	} else {
		var status uint64
		status, n, err = quicvarint.Parse(data)
		parsed += n
		m.ObjectStatus = ObjectStatus(status)
	}
	return
}
