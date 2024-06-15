package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type UnannounceMessage struct {
	TrackNamespace string
}

func (m *UnannounceMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(unannounceMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return buf
}

func (p *UnannounceMessage) parse(reader messageReader) (err error) {
	p.TrackNamespace, err = parseVarIntString(reader)
	return
}
