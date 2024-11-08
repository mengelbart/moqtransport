package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type FilterType uint64

const (
	FilterTypeLatestGroup FilterType = iota + 1
	FilterTypeLatestObject
	FilterTypeAbsoluteStart
	FilterTypeAbsoluteRange
)

func (f FilterType) append(buf []byte) []byte {
	switch f {
	case FilterTypeLatestGroup, FilterTypeLatestObject, FilterTypeAbsoluteStart, FilterTypeAbsoluteRange:
		return quicvarint.Append(buf, uint64(f))
	}
	return quicvarint.Append(buf, uint64(FilterTypeLatestGroup))
}

type SubscribeMessage struct {
	SubscribeID        uint64
	TrackAlias         uint64
	TrackNamespace     Tuple
	TrackName          []byte
	SubscriberPriority uint8
	GroupOrder         uint8
	FilterType         FilterType
	StartGroup         uint64
	StartObject        uint64
	EndGroup           uint64
	EndObject          uint64
	Parameters         Parameters
}

func (m SubscribeMessage) GetSubscribeID() uint64 {
	return m.SubscribeID
}

func (m SubscribeMessage) Type() controlMessageType {
	return messageTypeSubscribe
}

func (m *SubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, m.TrackName)
	buf = append(buf, m.SubscriberPriority)
	buf = append(buf, m.GroupOrder)
	buf = m.FilterType.append(buf)
	if m.FilterType == FilterTypeAbsoluteStart || m.FilterType == FilterTypeAbsoluteRange {
		buf = quicvarint.Append(buf, m.StartGroup)
		buf = quicvarint.Append(buf, m.StartObject)
	}
	if m.FilterType == FilterTypeAbsoluteRange {
		buf = quicvarint.Append(buf, m.EndGroup)
		buf = quicvarint.Append(buf, m.EndObject)
	}
	return m.Parameters.append(buf)
}

func (m *SubscribeMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackAlias, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackNamespace, n, err = parseTuple(data)
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

	filterType, n, err := quicvarint.Parse(data)
	if err != nil {
		return err
	}
	m.FilterType = FilterType(filterType)
	if m.FilterType == 0 || m.FilterType > 4 {
		return errInvalidFilterType
	}
	data = data[n:]

	if m.FilterType == FilterTypeAbsoluteStart || m.FilterType == FilterTypeAbsoluteRange {
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
	}

	if m.FilterType == FilterTypeAbsoluteRange {
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
	}
	m.Parameters = Parameters{}
	return m.Parameters.parse(data, versionSpecificParameterTypes)
}
