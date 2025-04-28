package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeErrorMessage struct {
	RequestID    uint64
	ErrorCode    uint64
	ReasonPhrase string
	TrackAlias   uint64
}

func (m *SubscribeErrorMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "subscribe_error"),
		slog.Uint64("request_id", m.RequestID),
		slog.Uint64("error_code", m.ErrorCode),
		slog.Uint64("track_alias", m.TrackAlias),
		slog.String("reason", m.ReasonPhrase),
		slog.Any("reason_bytes", []byte(m.ReasonPhrase)),
	)
}

func (m SubscribeErrorMessage) Type() controlMessageType {
	return messageTypeSubscribeError
}

func (m *SubscribeErrorMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = quicvarint.Append(buf, uint64(m.ErrorCode))
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	buf = quicvarint.Append(buf, m.TrackAlias)
	return buf
}

func (m *SubscribeErrorMessage) parse(_ Version, data []byte) (err error) {
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

	reasonPhrase, n, err := parseVarIntBytes(data)
	if err != nil {
		return err
	}
	m.ReasonPhrase = string(reasonPhrase)
	data = data[n:]

	m.TrackAlias, _, err = quicvarint.Parse(data)
	return err
}
