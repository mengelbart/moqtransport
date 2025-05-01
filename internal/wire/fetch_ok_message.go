package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type FetchOkMessage struct {
	RequestID           uint64
	GroupOrder          uint8
	EndOfTrack          uint8
	EndLocation         Location
	SubscribeParameters Parameters
}

func (m *FetchOkMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "fetch_ok"),
		slog.Uint64("request_id", m.RequestID),
		slog.Any("group_order", m.GroupOrder),
		slog.Any("end_of_track", m.EndOfTrack),
		slog.Uint64("largest_group_id", m.EndLocation.Group),
		slog.Uint64("largest_object_id", m.EndLocation.Object),
		slog.Uint64("number_of_parameters", uint64(len(m.SubscribeParameters))),
	}
	if len(m.SubscribeParameters) > 0 {
		attrs = append(attrs,
			slog.Any("subscribe_parameters", m.SubscribeParameters),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m FetchOkMessage) Type() controlMessageType {
	return messageTypeFetchOk
}

func (m *FetchOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = append(buf, m.GroupOrder)
	buf = append(buf, m.EndOfTrack)
	buf = m.EndLocation.append(buf)
	return m.SubscribeParameters.append(buf)
}

func (m *FetchOkMessage) parse(v Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.GroupOrder = data[0]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	m.EndOfTrack = data[1]
	data = data[2:]

	n, err = m.EndLocation.parse(v, data)
	if err != nil {
		return err
	}
	data = data[n:]

	return m.SubscribeParameters.parse(data)
}
