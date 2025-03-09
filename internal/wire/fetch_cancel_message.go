package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type FetchCancelMessage struct {
	SubscribeID uint64
}

func (m *FetchCancelMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "fetch_cancel"),
		slog.Uint64("subscribe_id", m.SubscribeID),
	)
}

func (m FetchCancelMessage) Type() controlMessageType {
	return messageTypeFetchCancel
}

func (m *FetchCancelMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.SubscribeID)
}

func (m *FetchCancelMessage) parse(data []byte) (err error) {
	m.SubscribeID, _, err = quicvarint.Parse(data)
	return err
}
