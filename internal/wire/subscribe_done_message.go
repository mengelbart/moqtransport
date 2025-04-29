package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeDoneMessage struct {
	RequestID    uint64
	StatusCode   uint64
	StreamCount  uint64
	ReasonPhrase string
}

func (m *SubscribeDoneMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "subscribe_done"),
		slog.Uint64("subscribe_id", m.RequestID),
		slog.Uint64("status_code", m.StatusCode),
		slog.Uint64("stream_count", m.StreamCount),
		slog.String("reason", m.ReasonPhrase),
	)
}

func (m SubscribeDoneMessage) Type() controlMessageType {
	return messageTypeSubscribeDone
}

func (m *SubscribeDoneMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = quicvarint.Append(buf, m.StreamCount)
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	return buf
}

func (m *SubscribeDoneMessage) parse(_ Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.StatusCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.StreamCount, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	reasonPhrase, _, err := parseVarIntBytes(data)
	if err != nil {
		return
	}
	m.ReasonPhrase = string(reasonPhrase)
	return nil
}
