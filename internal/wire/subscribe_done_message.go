package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeDoneMessage struct {
	SubscribeID  uint64
	StatusCode   uint64
	StreamCount  uint64
	ReasonPhrase string
}

func (m SubscribeDoneMessage) Type() controlMessageType {
	return messageTypeSubscribeDone
}

func (m *SubscribeDoneMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = quicvarint.Append(buf, m.StreamCount)
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	return buf
}

func (m *SubscribeDoneMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
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
