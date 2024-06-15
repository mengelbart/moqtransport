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

func (m *SubscribeDoneMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeDoneMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	if m.ContentExists {
		buf = append(buf, 1)
		buf = quicvarint.Append(buf, m.FinalGroup)
		buf = quicvarint.Append(buf, m.FinalObject)
		return buf
	}
	buf = append(buf, 0)
	return buf
}

func (m *SubscribeDoneMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.StatusCode, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.ReasonPhrase, err = parseVarIntString(reader)
	if err != nil {
		return
	}
	var contentExistsByte byte
	contentExistsByte, err = reader.ReadByte()
	if err != nil {
		return
	}
	switch contentExistsByte {
	case 0:
		m.ContentExists = false
	case 1:
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
	return
}
