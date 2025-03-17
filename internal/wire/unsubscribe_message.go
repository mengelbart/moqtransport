package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type UnsubscribeMessage struct {
	SubscribeID uint64
}

func (m *UnsubscribeMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "unsubscribe"),
		slog.Uint64("subscribe_id", m.SubscribeID),
	)
}

func (m UnsubscribeMessage) Type() controlMessageType {
	return messageTypeUnsubscribe
}

func (m *UnsubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	return buf
}

func (m *UnsubscribeMessage) parse(_ Version, data []byte) (err error) {
	m.SubscribeID, _, err = quicvarint.Parse(data)
	return err
}
