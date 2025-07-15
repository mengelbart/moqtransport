package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type SubscribeAnnouncesMessage struct {
	RequestID            uint64
	TrackNamespacePrefix Tuple
	Parameters           KVPList
}

func (m *SubscribeAnnouncesMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "subscribe_announces"),
		slog.Any("track_namespace_prefix", m.TrackNamespacePrefix),
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	}
	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("parameters", m.Parameters),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeAnnouncesMessage) Type() controlMessageType {
	return messageTypeSubscribeNamespace
}

func (m *SubscribeAnnouncesMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = m.TrackNamespacePrefix.append(buf)
	return m.Parameters.appendNum(buf)
}

func (m *SubscribeAnnouncesMessage) parse(_ Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackNamespacePrefix, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = KVPList{}
	return m.Parameters.parseNum(data)
}
