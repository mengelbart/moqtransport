package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type FetchErrorMessage struct {
	SubscribeID  uint64
	ErrorCode    uint64
	ReasonPhrase string
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

	reasonPhrase, n, err := parseVarIntBytes(data)
	if err != nil {
		return err
	}
	m.ReasonPhrase = string(reasonPhrase)
	return nil
}
