package wire

import (
	"log/slog"
	"maps"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeUpdateMessage struct {
	RequestID          uint64
	StartLocation      Location
	EndGroup           uint64
	SubscriberPriority uint8
	Forward            uint8
	Parameters         Parameters
}

func (m *SubscribeUpdateMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "subscribe_update"),
		slog.Uint64("subscribe_id", m.RequestID),
		slog.Uint64("start_group", m.StartLocation.Group),
		slog.Uint64("start_object", m.StartLocation.Object),
		slog.Uint64("end_group", m.EndGroup),
		slog.Any("subscriber_priority", m.SubscriberPriority),
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	}
	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", slices.Collect(maps.Values(m.Parameters))),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeUpdateMessage) Type() controlMessageType {
	return messageTypeSubscribeUpdate
}

func (m *SubscribeUpdateMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = m.StartLocation.append(buf)
	buf = quicvarint.Append(buf, m.EndGroup)
	buf = append(buf, m.SubscriberPriority)
	buf = append(buf, m.Forward)
	return m.Parameters.append(buf)
}

func (m *SubscribeUpdateMessage) parse(v Version, data []byte) (err error) {
	var n int

	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	n, err = m.StartLocation.parse(v, data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.EndGroup, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.SubscriberPriority = data[0]
	m.Forward = data[1]
	if m.Forward > 2 {
		return errInvalidForwardFlag
	}
	data = data[2:]

	m.Parameters = Parameters{}
	return m.Parameters.parseVersionSpecificParameters(data)
}
