package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribesBlockedMessage struct {
	MaximumSubscribeID uint64
}

func (m *SubscribesBlockedMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "subscribes_blocked"),
		slog.Uint64("max_subscribe_id", m.MaximumSubscribeID),
	)
}

func (m SubscribesBlockedMessage) Type() controlMessageType {
	return messageTypeRequestsBlocked
}

func (m *SubscribesBlockedMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.MaximumSubscribeID)
}

func (m *SubscribesBlockedMessage) parse(_ Version, data []byte) (err error) {
	m.MaximumSubscribeID, _, err = quicvarint.Parse(data)
	return err
}
