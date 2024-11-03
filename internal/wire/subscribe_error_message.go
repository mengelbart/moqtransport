package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeErrorMessage struct {
	SubscribeID  uint64
	ErrorCode    uint64
	ReasonPhrase string
	TrackAlias   uint64
}

func (m SubscribeErrorMessage) GetSubscribeID() uint64 {
	return m.SubscribeID
}

func (m SubscribeErrorMessage) Type() controlMessageType {
	return messageTypeSubscribeError
}

func (m *SubscribeErrorMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, uint64(m.ErrorCode))
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	buf = quicvarint.Append(buf, m.TrackAlias)
	return buf
}

func (m *SubscribeErrorMessage) parse(data []byte) (err error) {
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
	data = data[n:]

	m.TrackAlias, _, err = quicvarint.Parse(data)
	return err
}
