package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type SubscribeAnnouncesOkMessage struct {
	RequestID uint64
}

func (m *SubscribeAnnouncesOkMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "subscribe_announces_ok"),
	)
}

func (m SubscribeAnnouncesOkMessage) Type() controlMessageType {
	return messageTypeSubscribeNamespaceOk
}

func (m *SubscribeAnnouncesOkMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.RequestID)
}

func (m *SubscribeAnnouncesOkMessage) parse(_ Version, data []byte) (err error) {
	m.RequestID, _, err = quicvarint.Parse(data)
	return err
}
