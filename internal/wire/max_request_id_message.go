package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type MaxRequestIDMessage struct {
	RequestID uint64
}

func (m *MaxRequestIDMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "max_subscribe_id"),
		slog.Uint64("max_subscribe_id", m.RequestID),
	)
}

func (m MaxRequestIDMessage) Type() controlMessageType {
	return messageTypeMaxRequestID
}

func (m *MaxRequestIDMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.RequestID)
}

func (m *MaxRequestIDMessage) parse(_ Version, data []byte) (err error) {
	m.RequestID, _, err = quicvarint.Parse(data)
	return err
}
