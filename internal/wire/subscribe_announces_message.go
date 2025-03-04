package wire

import (
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/slices"
)

var _ slog.LogValuer = (*SubscribeAnnouncesMessage)(nil)

// TODO: Add tests
type SubscribeAnnouncesMessage struct {
	TrackNamespacePrefix Tuple
	Parameters           Parameters
}

func (m *SubscribeAnnouncesMessage) LogValue() slog.Value {
	ps := []Parameter{}
	for _, v := range m.Parameters {
		ps = append(ps, v)
	}
	attrs := []slog.Attr{
		slog.String("type", "subscribe_announces"),
		slog.Any("track_namespace_prefix", m.TrackNamespacePrefix),
		slog.Uint64("number_of_parameters", uint64(len(ps))),
	}
	if len(ps) > 0 {
		attrs = append(attrs,
			slog.Any("parameters", slices.Collect(slices.Map(ps, func(e Parameter) any { return e }))),
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

func (m *SubscribeAnnouncesMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespacePrefix, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data, versionSpecificParameterTypes)
}
