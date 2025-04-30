package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type UnsubscribeMessage struct {
	RequestID uint64
}

func (m *UnsubscribeMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "unsubscribe"),
		slog.Uint64("request_id", m.RequestID),
	)
}

func (m UnsubscribeMessage) Type() controlMessageType {
	return messageTypeUnsubscribe
}

func (m *UnsubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	return buf
}

func (m *UnsubscribeMessage) parse(_ Version, data []byte) (err error) {
	m.RequestID, _, err = quicvarint.Parse(data)
	return err
}
