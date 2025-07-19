package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type PublishOkMessage struct {
	RequestID          uint64
	Forward            uint8
	SubscriberPriority uint8
	GroupOrder         uint8
	FilterType         FilterType
	Start              Location
	EndGroup           uint64
	Parameters         KVPList
}

func (m *PublishOkMessage) LogValue() slog.Value {
	return slog.GroupValue()
}

func (m *PublishOkMessage) Type() controlMessageType {
	return messageTypePublish
}

func (m *PublishOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = append(buf, m.Forward)
	buf = append(buf, m.SubscriberPriority)
	buf = append(buf, m.GroupOrder)
	buf = m.FilterType.append(buf)
	if m.FilterType == FilterTypeAbsoluteStart || m.FilterType == FilterTypeAbsoluteRange {
		buf = m.Start.append(buf)
	}
	if m.FilterType == FilterTypeAbsoluteRange {
		buf = quicvarint.Append(buf, m.EndGroup)
	}
	return m.Parameters.append(buf)
}

func (m *PublishOkMessage) parse(v Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < 3 {
		return errLengthMismatch
	}
	m.Forward = data[0]
	if m.Forward > 1 {
		return errInvalidForwardFlag
	}
	m.SubscriberPriority = data[1]
	m.GroupOrder = data[2]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
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
		n, err = m.Start.parse(v, data)
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
