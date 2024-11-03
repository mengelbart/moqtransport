package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeDoneMessage struct {
	SubscribeID   uint64
	StatusCode    uint64
	ReasonPhrase  string
	ContentExists bool
	FinalGroup    uint64
	FinalObject   uint64
}

func (m SubscribeDoneMessage) Type() controlMessageType {
	return messageTypeSubscribeDone
}

func (m *SubscribeDoneMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	if m.ContentExists {
		buf = append(buf, 1)
		buf = quicvarint.Append(buf, m.FinalGroup)
		buf = quicvarint.Append(buf, m.FinalObject)
		return buf
	}
	buf = append(buf, 0)
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

	reasonPhrase, n, err := parseVarIntBytes(data)
	if err != nil {
		return
	}
	m.ReasonPhrase = string(reasonPhrase)
	data = data[n:]

	if len(data) == 0 {
		return errLengthMismatch
	}
	if data[0] != 0 && data[0] != 1 {
		return errInvalidContentExistsByte
	}
	m.ContentExists = data[0] == 1
	if !m.ContentExists {
		return
	}
	data = data[1:]

	m.FinalGroup, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.FinalObject, _, err = quicvarint.Parse(data)
	return err
}
