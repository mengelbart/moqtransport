package wire

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type StreamHeaderGroupObject struct {
	ObjectID      uint64
	ObjectPayload []byte
}

func (m *StreamHeaderGroupObject) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func (m *StreamHeaderGroupObject) parse(reader messageReader) (err error) {
	m.ObjectID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	var objectLen uint64
	objectLen, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.ObjectPayload = make([]byte, objectLen)
	_, err = io.ReadFull(reader, m.ObjectPayload)
	return
}
