package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type FetchHeaderMessage struct {
	RequestID uint64
}

func (m *FetchHeaderMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(StreamTypeFetch))
	return quicvarint.Append(buf, m.RequestID)
}

func (m *FetchHeaderMessage) parse(reader messageReader) (err error) {
	m.RequestID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	return
}
