package wire

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type StreamHeaderTrackObject struct {
	GroupID       uint64
	ObjectID      uint64
	ObjectStatus  ObjectStatus
	ObjectPayload []byte
}

func (m *StreamHeaderTrackObject) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	if len(m.ObjectPayload) > 0 {
		buf = append(buf, m.ObjectPayload...)
	} else {
		buf = quicvarint.Append(buf, uint64(m.ObjectStatus))
	}
	return buf
}

func (m *StreamHeaderTrackObject) parse(reader messageReader) (err error) {
	m.GroupID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.ObjectID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	var objectLen uint64
	objectLen, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	if objectLen > 0 {
		m.ObjectPayload = make([]byte, objectLen)
		_, err = io.ReadFull(reader, m.ObjectPayload)
		return
	}
	var status uint64
	status, err = quicvarint.Read(reader)
	m.ObjectStatus = ObjectStatus(status)
	return
}
