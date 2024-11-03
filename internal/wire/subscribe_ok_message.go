package wire

import (
	"time"

	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeOkMessage struct {
	SubscribeID   uint64
	Expires       time.Duration
	GroupOrder    uint8
	ContentExists bool
	FinalGroup    uint64
	FinalObject   uint64
}

func (m SubscribeOkMessage) GetSubscribeID() uint64 {
	return m.SubscribeID
}

func (m SubscribeOkMessage) Type() controlMessageType {
	return messageTypeSubscribeOk
}

func (m *SubscribeOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, uint64(m.Expires))
	buf = append(buf, m.GroupOrder)
	if m.ContentExists {
		buf = append(buf, 1) // ContentExists=true
		buf = quicvarint.Append(buf, m.FinalGroup)
		buf = quicvarint.Append(buf, m.FinalObject)
		return buf
	}
	buf = append(buf, 0) // ContentExists=false
	return buf
}

func (m *SubscribeOkMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	expires, n, err := quicvarint.Parse(data)
	if err != nil {
		return
	}
	m.Expires = time.Duration(expires) * time.Millisecond
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.GroupOrder = data[0]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	if data[1] != 0 && data[1] != 1 {
		return errInvalidContentExistsByte
	}
	m.ContentExists = data[1] == 1
	if !m.ContentExists {
		return
	}
	data = data[2:]

	m.FinalGroup, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.FinalObject, _, err = quicvarint.Parse(data)
	return err
}
