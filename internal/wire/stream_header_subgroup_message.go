package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type StreamHeaderSubgroupMessage struct {
	TrackAlias        uint64
	GroupID           uint64
	SubgroupID        uint64
	PublisherPriority uint8
}

func (m *StreamHeaderSubgroupMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(StreamTypeSubgroup))
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.SubgroupID)
	return append(buf, m.PublisherPriority)
}

func (m *StreamHeaderSubgroupMessage) parse(reader messageReader) (err error) {
	m.TrackAlias, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.GroupID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.SubgroupID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.PublisherPriority, err = reader.ReadByte()
	return
}
