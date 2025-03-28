package wire

import (
	"log/slog"

	"maps"

	"github.com/mengelbart/moqtransport/internal/slices"
)

// TODO: Add tests
type SubscribeAnnouncesMessage struct {
	TrackNamespacePrefix Tuple
	Parameters           Parameters
}

func (m *SubscribeAnnouncesMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "subscribe_announces"),
		slog.Any("track_namespace_prefix", m.TrackNamespacePrefix),
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	}
	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("parameters", slices.Collect(maps.Values(m.Parameters))),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeAnnouncesMessage) Type() controlMessageType {
	return messageTypeSubscribeAnnounces
}

func (m *SubscribeAnnouncesMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespacePrefix.append(buf)
	return m.Parameters.append(buf)
}

func (m *SubscribeAnnouncesMessage) parse(_ Version, data []byte) (err error) {
	var n int
	m.TrackNamespacePrefix, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parseVersionSpecificParameters(data)
}
