package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceOkMessage struct {
	TrackNamespace string
}

func (m AnnounceOkMessage) GetTrackNamespace() string {
	return m.TrackNamespace
}

func (m *AnnounceOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceOkMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return buf
}

func (m *AnnounceOkMessage) parse(reader messageReader) (err error) {
	m.TrackNamespace, err = parseVarIntString(reader)
	return
}
