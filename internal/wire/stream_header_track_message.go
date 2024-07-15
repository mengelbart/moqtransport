package wire

import "github.com/quic-go/quic-go/quicvarint"

type StreamHeaderTrackMessage struct {
	SubscribeID       uint64
	TrackAlias        uint64
	PublisherPriority uint8
}

func (m *StreamHeaderTrackMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(StreamHeaderTrackMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = append(buf, m.PublisherPriority)
	return buf
}

func (m *StreamHeaderTrackMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.TrackAlias, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.PublisherPriority, err = reader.ReadByte()
	return
}
