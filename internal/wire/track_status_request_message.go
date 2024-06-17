package wire

import "github.com/quic-go/quic-go/quicvarint"

type TrackStatusRequestMessage struct {
	TrackNamespace string
	TrackName      string
}

func (m *TrackStatusRequestMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(trackStatusRequestMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return appendVarIntString(buf, m.TrackName)
}

func (m *TrackStatusRequestMessage) parse(reader messageReader) (err error) {
	m.TrackNamespace, err = parseVarIntString(reader)
	if err != nil {
		return
	}
	m.TrackName, err = parseVarIntString(reader)
	return
}
