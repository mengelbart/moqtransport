package wire

import (
	"log/slog"

	"github.com/mengelbart/qlog"
	"github.com/quic-go/quic-go/quicvarint"
)

const (
	FetchTypeStandalone = 0x01
	FetchTypeJoining    = 0x02
)

// TODO: Add tests
type FetchMessage struct {
	RequestID            uint64
	SubscriberPriority   uint8
	GroupOrder           uint8
	FetchType            uint64
	TrackNamespace       Tuple
	TrackName            []byte
	StartGroup           uint64
	StartObject          uint64
	EndGroup             uint64
	EndObject            uint64
	JoiningSubscribeID   uint64
	PrecedingGroupOffset uint64
	Parameters           Parameters
}

// Attrs implements moqt.ControlMessage.
func (m *FetchMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "fetch"),
		slog.Uint64("request_id", m.RequestID),
		slog.Any("subscriber_priority", m.SubscriberPriority),
		slog.Any("group_order", m.GroupOrder),
		slog.Uint64("fetch_type", m.FetchType),
	}

	if m.FetchType == FetchTypeStandalone {
		attrs = append(attrs,
			slog.Any("track_namespace", m.TrackNamespace),
			slog.Any("track_name", qlog.RawInfo{
				Length:        uint64(len(m.TrackName)),
				PayloadLength: uint64(len(m.TrackName)),
				Data:          m.TrackName,
			}),
			slog.Uint64("start_group", m.StartGroup),
			slog.Uint64("start_object", m.StartObject),
			slog.Uint64("end_group", m.EndGroup),
			slog.Uint64("end_object", m.EndObject),
		)
	}
	if m.FetchType == FetchTypeJoining {
		attrs = append(attrs,
			slog.Uint64("joining_subscribe_id", m.JoiningSubscribeID),
			slog.Uint64("preceding_group_offset", m.PrecedingGroupOffset),
		)
	}

	attrs = append(attrs,
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	)

	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", m.Parameters),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m FetchMessage) Type() controlMessageType {
	return messageTypeFetch
}

func (m *FetchMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = append(buf, m.SubscriberPriority)
	buf = append(buf, m.GroupOrder)
	buf = quicvarint.Append(buf, m.FetchType)
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.StartGroup)
	buf = quicvarint.Append(buf, m.StartObject)
	buf = quicvarint.Append(buf, m.EndGroup)
	buf = quicvarint.Append(buf, m.EndObject)
	buf = quicvarint.Append(buf, m.JoiningSubscribeID)
	buf = quicvarint.Append(buf, m.PrecedingGroupOffset)
	return m.Parameters.append(buf)
}

func (m *FetchMessage) parse(_ Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
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

	m.FetchType, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if m.FetchType != FetchTypeStandalone && m.FetchType != FetchTypeJoining {
		return errInvalidFetchType
	}

	if m.FetchType == FetchTypeStandalone {
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
	}
	if m.FetchType == FetchTypeJoining {
		m.JoiningSubscribeID, n, err = quicvarint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]

		m.PrecedingGroupOffset, n, err = quicvarint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}

	m.Parameters = Parameters{}
	return m.Parameters.parse(data)
}
