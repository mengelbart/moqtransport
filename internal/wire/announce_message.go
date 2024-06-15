package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceMessage struct {
	TrackNamespace string
	Parameters     Parameters
}

func (m AnnounceMessage) GetTrackNamespace() string {
	return m.TrackNamespace
}

func (m *AnnounceMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return m.Parameters.append(buf)
}

func (m *AnnounceMessage) parse(reader messageReader) (err error) {
	m.TrackNamespace, err = parseVarIntString(reader)
	if err != nil {
		return err
	}
	m.Parameters = Parameters{}
	return m.Parameters.parse(reader)
}
