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

func (m *SubscribeErrorMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeErrorMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, uint64(m.ErrorCode))
	buf = appendVarIntString(buf, m.ReasonPhrase)
	buf = quicvarint.Append(buf, m.TrackAlias)
	return buf
}

func (m *SubscribeErrorMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.ErrorCode, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.ReasonPhrase, err = parseVarIntString(reader)
	if err != nil {
		return err
	}
	m.TrackAlias, err = quicvarint.Read(reader)
	return
}
