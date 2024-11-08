package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type FetchMessage struct {
	SubscribeID        uint64
	TrackNamspace      Tuple
	TrackName          []byte
	SubscriberPriority uint8
	GroupOrder         uint8
	StartGroup         uint64
	StartObject        uint64
	EndGroup           uint64
	EndObject          uint64
	Parameters         Parameters
}

func (m FetchMessage) Type() controlMessageType {
	return messageTypeFetch
}

func (m *FetchMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = m.TrackNamspace.append(buf)
	buf = appendVarIntBytes(buf, m.TrackName)
	buf = append(buf, m.SubscriberPriority)
	buf = append(buf, m.GroupOrder)
	buf = quicvarint.Append(buf, m.StartGroup)
	buf = quicvarint.Append(buf, m.StartObject)
	buf = quicvarint.Append(buf, m.EndGroup)
	buf = quicvarint.Append(buf, m.EndObject)
	return m.Parameters.append(buf)
}

func (m *FetchMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackNamspace, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackName, n, err = parseVarIntBytes(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.SubscriberPriority = data[0]
	m.GroupOrder = data[1]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	data = data[2:]

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

	m.Parameters = Parameters{}
	return m.Parameters.parse(data, versionSpecificParameterTypes)
}
