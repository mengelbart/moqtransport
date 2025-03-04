package wire

import (
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/slices"
)

var _ slog.LogValuer = (*AnnounceMessage)(nil)

type AnnounceMessage struct {
	TrackNamespace Tuple
	Parameters     Parameters
}

func (m *AnnounceMessage) LogValue() slog.Value {
	ps := []Parameter{}
	for _, v := range m.Parameters {
		ps = append(ps, v)
	}
	attrs := []slog.Attr{
		slog.String("type", "announce"),
		slog.Any("track_namespace", m.TrackNamespace),
		slog.Uint64("number_of_parameters", uint64(len(ps))),
	}
	if len(ps) > 0 {
		attrs = append(attrs,
			slog.Any("parameters", slices.Collect(slices.Map(ps, func(e Parameter) any { return e }))),
		)
	}
	return slog.GroupValue(attrs...)
}

func (m AnnounceMessage) GetTrackNamespace() string {
	return m.TrackNamespace.String()
}

func (m AnnounceMessage) Type() controlMessageType {
	return messageTypeAnnounce
}

func (m *AnnounceMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return m.Parameters.append(buf)
}

func (m *AnnounceMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data, versionSpecificParameterTypes)
}
