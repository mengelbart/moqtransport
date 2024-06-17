package wire

import "github.com/quic-go/quic-go/quicvarint"

type TrackStatusMessage struct {
	TrackNamespace string
	TrackName      string
	StatusCode     uint64
	LatestGroupID  uint64
	LatestObjectID uint64
}

func (m *TrackStatusMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(trackStatusMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = quicvarint.Append(buf, m.LatestGroupID)
	return quicvarint.Append(buf, m.LatestObjectID)
}

func (m *TrackStatusMessage) parse(reader messageReader) (err error) {
	m.TrackNamespace, err = parseVarIntString(reader)
	if err != nil {
		return
	}
	m.TrackName, err = parseVarIntString(reader)
	if err != nil {
		return
	}
	m.StatusCode, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.LatestGroupID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.LatestObjectID, err = quicvarint.Read(reader)
	return
}
