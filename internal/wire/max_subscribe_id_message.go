package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type MaxSubscribeIDMessage struct {
	SubscribeID uint64
}

func (m *MaxSubscribeIDMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "max_subscribe_id"),
		slog.Uint64("max_subscribe_id", m.SubscribeID),
	)
}

func (m MaxSubscribeIDMessage) Type() controlMessageType {
	return messageTypeMaxRequestID
}

func (m *MaxSubscribeIDMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.SubscribeID)
}

func (m *MaxSubscribeIDMessage) parse(_ Version, data []byte) (err error) {
	m.SubscribeID, _, err = quicvarint.Parse(data)
	return err
}
