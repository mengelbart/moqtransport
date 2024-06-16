package wire

import "github.com/quic-go/quic-go/quicvarint"

type StreamHeaderGroupMessage struct {
	SubscribeID     uint64
	TrackAlias      uint64
	GroupID         uint64
	ObjectSendOrder uint64
}

func (m *StreamHeaderGroupMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(StreamHeaderGroupMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectSendOrder)
	return buf
}

func (m *StreamHeaderGroupMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.TrackAlias, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.GroupID, err = quicvarint.Read(reader)
	if err != nil {
		return
	}
	m.ObjectSendOrder, err = quicvarint.Read(reader)
	return
}
