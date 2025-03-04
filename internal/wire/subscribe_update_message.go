package wire

import (
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/quic-go/quic-go/quicvarint"
)

var _ slog.LogValuer = (*SubscribeUpdateMessage)(nil)

type SubscribeUpdateMessage struct {
	SubscribeID        uint64
	StartGroup         uint64
	StartObject        uint64
	EndGroup           uint64
	SubscriberPriority uint8
	Parameters         Parameters
}

func (m *SubscribeUpdateMessage) LogValue() slog.Value {
	ps := []Parameter{}
	for _, v := range m.Parameters {
		ps = append(ps, v)
	}
	attrs := []slog.Attr{
		slog.String("type", "subscribe_update"),
		slog.Uint64("subscribe_id", m.SubscribeID),
		slog.Uint64("start_group", m.StartGroup),
		slog.Uint64("start_object", m.StartObject),
		slog.Uint64("end_group", m.EndGroup),
		slog.Any("subscriber_priority", m.SubscriberPriority),
		slog.Uint64("number_of_parameters", uint64(len(ps))),
	}
	if len(ps) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", slices.Collect(slices.Map(ps, func(e Parameter) any { return e }))),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeUpdateMessage) Type() controlMessageType {
	return messageTypeSubscribeUpdate
}

func (m *SubscribeUpdateMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.StartGroup)
	buf = quicvarint.Append(buf, m.StartObject)
	buf = quicvarint.Append(buf, m.EndGroup)
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

	if len(data) == 0 {
		return errLengthMismatch
	}
	m.SubscriberPriority = data[0]
	data = data[1:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data, versionSpecificParameterTypes)
}
