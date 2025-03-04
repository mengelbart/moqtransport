package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

var _ slog.LogValuer = (*FetchErrorMessage)(nil)

// TODO: Add tests
type FetchErrorMessage struct {
	SubscribeID  uint64
	ErrorCode    uint64
	ReasonPhrase string
}

func (m *FetchErrorMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "fetch_error"),
		slog.Uint64("subscribe_id", m.SubscribeID),
		slog.Uint64("error_code", m.ErrorCode),
		slog.String("reason", m.ReasonPhrase),
	)
}

func (m FetchErrorMessage) Type() controlMessageType {
	return messageTypeFetchError
}

func (m *FetchErrorMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.ErrorCode)
	return appendVarIntBytes(buf, []byte(m.ReasonPhrase))
}

func (m *FetchErrorMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
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
	if err != nil {
		return err
	}
	m.ReasonPhrase = string(reasonPhrase)
	return nil
}
