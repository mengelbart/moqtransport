package wire

import "github.com/quic-go/quic-go/quicvarint"

type TrackStatusMessage struct {
	TrackNamespace Tuple
	TrackName      string
	StatusCode     uint64
	LatestGroupID  uint64
	LatestObjectID uint64
}

func (m TrackStatusMessage) Type() controlMessageType {
	return messageTypeTrackStatus
}

func (m *TrackStatusMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, []byte(m.TrackName))
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = quicvarint.Append(buf, m.LatestGroupID)
	return quicvarint.Append(buf, m.LatestObjectID)
}

func (m *TrackStatusMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return
	}
	data = data[n:]

	trackName, n, err := parseVarIntBytes(data)
	if err != nil {
		return
	}
	m.TrackName = string(trackName)
	data = data[n:]

	m.StatusCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LatestGroupID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LatestObjectID, _, err = quicvarint.Parse(data)
	return err
}
