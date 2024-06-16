package wire

import "github.com/quic-go/quic-go/quicvarint"

type AnnounceCancelMessage struct {
	TrackNamespace string
}

func (m AnnounceCancelMessage) GetTrackNamespace() string {
	return m.TrackNamespace
}

func (m *AnnounceCancelMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceCancelMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return buf
}

func (m *AnnounceCancelMessage) parse(reader messageReader) (err error) {
	m.TrackNamespace, err = parseVarIntString(reader)
	return
}
