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
	TrackNamespace     string
	TrackName          string
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

func (m *SubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
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

func (m *SubscribeMessage) parse(reader messageReader) (err error) {
	m.SubscribeID, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.TrackAlias, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.TrackNamespace, err = parseVarIntString(reader)
	if err != nil {
		return err
	}
	m.TrackName, err = parseVarIntString(reader)
	if err != nil {
		return err
	}
	m.SubscriberPriority, err = reader.ReadByte()
	if err != nil {
		return err
	}
	m.GroupOrder, err = reader.ReadByte()
	if err != nil {
		return err
	}
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	ft, err := quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.FilterType = FilterType(ft)
	switch m.FilterType {
	case FilterTypeLatestGroup, FilterTypeLatestObject, FilterTypeAbsoluteStart, FilterTypeAbsoluteRange:
	default:
		return errInvalidFilterType
	}
	if m.FilterType == FilterTypeAbsoluteStart || m.FilterType == FilterTypeAbsoluteRange {
		m.StartGroup, err = quicvarint.Read(reader)
		if err != nil {
			return err
		}
		m.StartObject, err = quicvarint.Read(reader)
		if err != nil {
			return err
		}
	}
	if m.FilterType == FilterTypeAbsoluteRange {
		m.EndGroup, err = quicvarint.Read(reader)
		if err != nil {
			return err
		}
		m.EndObject, err = quicvarint.Read(reader)
		if err != nil {
			return err
		}
	}
	m.Parameters = Parameters{}
	return m.Parameters.parse(reader)
}
