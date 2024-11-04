package wire

import "github.com/quic-go/quic-go/quicvarint"

type TrackStatusMessage struct {
	TrackNamespace Tuple
	TrackName      string
	StatusCode     uint64
	LastGroupID    uint64
	LastObjectID   uint64
}

func (m TrackStatusMessage) Type() controlMessageType {
	return messageTypeTrackStatus
}

func (m *TrackStatusMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, []byte(m.TrackName))
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = quicvarint.Append(buf, m.LastGroupID)
	return quicvarint.Append(buf, m.LastObjectID)
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

	m.LastGroupID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LastObjectID, _, err = quicvarint.Parse(data)
	return err
}
