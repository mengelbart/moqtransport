package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type UnsubscribeMessage struct {
	SubscribeID uint64
}

func (m *UnsubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(unsubscribeMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	return buf
}

func (m *UnsubscribeMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	return
}
