package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceMessage struct {
	RequestID      uint64
	TrackNamespace Tuple
	Parameters     KVPList
}

func (m *AnnounceMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "announce"),
		slog.Any("track_namespace", m.TrackNamespace),
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	}
	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("parameters", m.Parameters),
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
	buf = quicvarint.Append(buf, m.RequestID)
	buf = m.TrackNamespace.append(buf)
	return m.Parameters.appendNum(buf)
}

func (m *AnnounceMessage) parse(_ Version, data []byte) (err error) {
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

	m.Parameters = KVPList{}
	return m.Parameters.parseNum(data)
}
