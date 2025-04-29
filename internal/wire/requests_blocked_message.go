package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type RequestsBlockedMessage struct {
	MaximumRequestID uint64
}

func (m *RequestsBlockedMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "subscribes_blocked"),
		slog.Uint64("max_subscribe_id", m.MaximumRequestID),
	)
}

func (m RequestsBlockedMessage) Type() controlMessageType {
	return messageTypeRequestsBlocked
}

func (m *RequestsBlockedMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.MaximumRequestID)
}

func (m *RequestsBlockedMessage) parse(_ Version, data []byte) (err error) {
	m.MaximumRequestID, _, err = quicvarint.Parse(data)
	return err
}
