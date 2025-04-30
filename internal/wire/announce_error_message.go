package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceErrorMessage struct {
	RequestID    uint64
	ErrorCode    uint64
	ReasonPhrase string
}

func (m *AnnounceErrorMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "announce_error"),
		slog.Uint64("error_code", m.ErrorCode),
		slog.String("reason", m.ReasonPhrase),
	)
}

func (m AnnounceErrorMessage) Type() controlMessageType {
	return messageTypeAnnounceError
}

func (m *AnnounceErrorMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	return buf
}

func (m *AnnounceErrorMessage) parse(_ Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.ErrorCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	reasonPhrase, _, err := parseVarIntBytes(data)
	m.ReasonPhrase = string(reasonPhrase)
	return err
}
