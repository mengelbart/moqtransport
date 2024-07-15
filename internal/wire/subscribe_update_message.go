package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeUpdateMessage struct {
	SubscribeID        uint64
	StartGroup         uint64
	StartObject        uint64
	EndGroup           uint64
	EndObject          uint64
	SubscriberPriority uint8
	Parameters         Parameters
}

func (m *SubscribeUpdateMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeUpdateMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.StartGroup)
	buf = quicvarint.Append(buf, m.StartObject)
	buf = quicvarint.Append(buf, m.EndGroup)
	buf = quicvarint.Append(buf, m.EndObject)
	buf = append(buf, m.SubscriberPriority)
	return m.Parameters.append(buf)
}

func (m *SubscribeUpdateMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.StartGroup, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.StartObject, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.EndGroup, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.EndObject, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.SubscriberPriority, err = reader.ReadByte()
	if err != nil {
		return err
	}
	m.Parameters = Parameters{}
	return m.Parameters.parse(reader)
}
