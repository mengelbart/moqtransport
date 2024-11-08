package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type FetchHeaderMessage struct {
	SubscribeID uint64
}

func (m *FetchHeaderMessage) Append(buf []byte) []byte {
	return quicvarint.Append(buf, m.SubscribeID)
}

func (m *FetchHeaderMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	return
}
