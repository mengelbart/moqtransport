package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type MaxSubscribeIDMessage struct {
	SubscribeID uint64
}

func (m MaxSubscribeIDMessage) Type() controlMessageType {
	return messageTypeMaxSubscribeID
}

func (m *MaxSubscribeIDMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.SubscribeID)
}

func (m *MaxSubscribeIDMessage) parse(data []byte) (err error) {
	m.SubscribeID, _, err = quicvarint.Parse(data)
	return err
}
