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

func (m SubscribeUpdateMessage) Type() controlMessageType {
	return messageTypeSubscribeUpdate
}

func (m *SubscribeUpdateMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.StartGroup)
	buf = quicvarint.Append(buf, m.StartObject)
	buf = quicvarint.Append(buf, m.EndGroup)
	buf = quicvarint.Append(buf, m.EndObject)
	buf = append(buf, m.SubscriberPriority)
	return m.Parameters.append(buf)
}

func (m *SubscribeUpdateMessage) parse(data []byte) (err error) {
	var n int

	m.SubscribeID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.StartGroup, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.StartObject, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.EndGroup, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.EndObject, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) == 0 {
		return errLengthMismatch
	}
	m.SubscriberPriority = data[0]
	data = data[1:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data)
}
