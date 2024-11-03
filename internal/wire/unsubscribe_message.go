package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type UnsubscribeMessage struct {
	SubscribeID uint64
}

func (m UnsubscribeMessage) Type() controlMessageType {
	return messageTypeUnsubscribe
}

func (m *UnsubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	return buf
}

func (m *UnsubscribeMessage) parse(data []byte) (err error) {
	m.SubscribeID, _, err = quicvarint.Parse(data)
	return err
}
