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

func (m *SubscribeOkMessage) Append(buf []byte) []byte {
	if m.GroupOrder == 0 {
		panic(errInvalidGroupOrder)
	}
	buf = quicvarint.Append(buf, uint64(subscribeOkMessageType))
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

func (m *SubscribeOkMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	var expires uint64
	expires, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.Expires = time.Duration(expires) * time.Millisecond
	m.GroupOrder, err = reader.ReadByte()
	if err != nil {
		return err
	}
	if m.GroupOrder == 0 || m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	var contentExistsByte byte
	contentExistsByte, err = reader.ReadByte()
	if err != nil {
		return
	}
	switch contentExistsByte {
	case byte(0):
		m.ContentExists = false
	case byte(1):
		m.ContentExists = true
	default:
		return errInvalidContentExistsByte
	}
	if !m.ContentExists {
		return
	}
	m.FinalGroup, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.FinalObject, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	return
}
