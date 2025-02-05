package wire

import "github.com/quic-go/quic-go/quicvarint"

type SubscribesBlocked struct {
	MaximumSubscribeID uint64
}

func (m SubscribesBlocked) Type() controlMessageType {
	return messageTypeSubscribesBlocked
}

func (m *SubscribesBlocked) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.MaximumSubscribeID)
}

func (m *SubscribesBlocked) parse(data []byte) (err error) {
	m.MaximumSubscribeID, _, err = quicvarint.Parse(data)
	return err
}
