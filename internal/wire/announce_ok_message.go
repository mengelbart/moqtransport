package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceOkMessage struct {
	RequestID uint64
}

func (m *AnnounceOkMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "announce_ok"),
	)
}

func (m AnnounceOkMessage) Type() controlMessageType {
	return messageTypeAnnounceOk
}

func (m *AnnounceOkMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.RequestID)
}

func (m *AnnounceOkMessage) parse(_ Version, data []byte) (err error) {
	m.RequestID, _, err = quicvarint.Parse(data)
	return err
}
