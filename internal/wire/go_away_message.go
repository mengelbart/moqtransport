package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type GoAwayMessage struct {
	NewSessionURI string
}

func (m *GoAwayMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(goAwayMessageType))
	buf = appendVarIntString(buf, m.NewSessionURI)
	return buf
}

func (m *GoAwayMessage) parse(reader messageReader) (err error) {
	m.NewSessionURI, err = parseVarIntString(reader)
	return
}
