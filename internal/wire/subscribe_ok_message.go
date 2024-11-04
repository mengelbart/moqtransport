package wire

import (
	"time"

	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeOkMessage struct {
	SubscribeID     uint64
	Expires         time.Duration
	GroupOrder      uint8
	ContentExists   bool
	LargestGroupID  uint64
	LargestObjectID uint64
	Parameters      Parameters
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
		buf = quicvarint.Append(buf, m.LargestGroupID)
		buf = quicvarint.Append(buf, m.LargestObjectID)
		return m.Parameters.append(buf)
	}
	buf = append(buf, 0) // ContentExists=false
	return m.Parameters.append(buf)
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
	data = data[2:]

	if !m.ContentExists {
		m.Parameters = Parameters{}
		return m.Parameters.parse(data)
	}

	m.LargestGroupID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LargestObjectID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data)
}
