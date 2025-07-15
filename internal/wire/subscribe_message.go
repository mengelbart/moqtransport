package wire

import (
	"fmt"
	"log/slog"

	"github.com/mengelbart/qlog"
	"github.com/quic-go/quic-go/quicvarint"
)

type FilterType uint64

const (
	FilterTypeLatestObject   FilterType = 0x02
	FilterTypeNextGroupStart FilterType = 0x01
	FilterTypeAbsoluteStart  FilterType = 0x03
	FilterTypeAbsoluteRange  FilterType = 0x04
)

// String returns a human-readable description of the FilterType.
func (f FilterType) String() string {
	switch f {
	case FilterTypeLatestObject:
		return "LatestObject"
	case FilterTypeNextGroupStart:
		return "NextGroupStart"
	case FilterTypeAbsoluteStart:
		return "AbsoluteStart"
	case FilterTypeAbsoluteRange:
		return "AbsoluteRange"
	default:
		return fmt.Sprintf("Unknown(%d)", uint64(f))
	}
}

// make sure we always set a valid value instead of the zero value (0)
func (f FilterType) append(buf []byte) []byte {
	switch f {
	case FilterTypeLatestObject, FilterTypeNextGroupStart, FilterTypeAbsoluteStart, FilterTypeAbsoluteRange:
		return quicvarint.Append(buf, uint64(f))
	}
	return quicvarint.Append(buf, uint64(FilterTypeNextGroupStart))
}

type GroupOrder uint8

const (
	// GroupOrderNone indicates no specific ordering preference.
	GroupOrderNone GroupOrder = 0x0

	// GroupOrderAscending indicates groups should be delivered in ascending order.
	GroupOrderAscending GroupOrder = 0x1

	// GroupOrderDescending indicates groups should be delivered in descending order.
	GroupOrderDescending GroupOrder = 0x2
)

// String returns a human-readable description of the GroupOrder.
func (g GroupOrder) String() string {
	switch g {
	case GroupOrderNone:
		return "None"
	case GroupOrderAscending:
		return "Ascending"
	case GroupOrderDescending:
		return "Descending"
	default:
		return fmt.Sprintf("Invalid(%d)", uint8(g))
	}
}

type SubscribeMessage struct {
	RequestID          uint64
	TrackNamespace     Tuple
	TrackName          []byte
	SubscriberPriority uint8
	GroupOrder         GroupOrder
	Forward            uint8
	FilterType         FilterType
	StartLocation      Location
	EndGroup           uint64
	Parameters         KVPList
}

func (m *SubscribeMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "subscribe"),
		slog.Uint64("request_id", m.RequestID),
		slog.Any("track_namespace", m.TrackNamespace),
		slog.Any("track_name", qlog.RawInfo{
			Length:        uint64(len(m.TrackName)),
			PayloadLength: uint64(len(m.TrackName)),
			Data:          m.TrackName,
		}),
		slog.Any("subscriber_priority", m.SubscriberPriority),
		slog.Any("group_order", m.GroupOrder),
		slog.Any("forward", m.Forward),
		slog.Any("filter_type", m.FilterType),
	}
	if m.FilterType == FilterTypeAbsoluteStart || m.FilterType == FilterTypeAbsoluteRange {
		attrs = append(attrs,
			slog.Uint64("start_group", m.StartLocation.Group),
			slog.Uint64("start_object", m.StartLocation.Object),
		)
	}
	if m.FilterType == FilterTypeAbsoluteRange {
		attrs = append(attrs,
			slog.Uint64("end_group", m.EndGroup),
		)
	}
	attrs = append(attrs,
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	)
	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("subscribe_parameters", m.Parameters),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeMessage) Type() controlMessageType {
	return messageTypeSubscribe
}

func (m *SubscribeMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, m.TrackName)
	buf = append(buf, m.SubscriberPriority)
	buf = append(buf, byte(m.GroupOrder))
	buf = append(buf, m.Forward)
	buf = m.FilterType.append(buf)
	if m.FilterType == FilterTypeAbsoluteStart || m.FilterType == FilterTypeAbsoluteRange {
		buf = m.StartLocation.append(buf)
	}
	if m.FilterType == FilterTypeAbsoluteRange {
		buf = quicvarint.Append(buf, m.EndGroup)
	}
	return m.Parameters.appendNum(buf)
}

func (m *SubscribeMessage) parse(v Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
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

	if len(data) < 3 {
		return errLengthMismatch
	}
	m.SubscriberPriority = data[0]
	m.GroupOrder = GroupOrder(data[1])
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	m.Forward = data[2]
	if m.Forward > 1 {
		return errInvalidForwardFlag
	}
	data = data[3:]

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
		n, err = m.StartLocation.parse(v, data)
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
	}
	m.Parameters = KVPList{}
	return m.Parameters.parseNum(data)
}
