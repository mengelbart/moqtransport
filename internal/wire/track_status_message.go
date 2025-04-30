package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type TrackStatusMessage struct {
	RequestID       uint64
	StatusCode      uint64
	LargestLocation Location
	Parameters      Parameters
}

func (m *TrackStatusMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "track_status"),
		slog.Uint64("status_code", m.StatusCode),
		slog.Uint64("last_group_id", m.LargestLocation.Group),
		slog.Uint64("last_object_id", m.LargestLocation.Object),
	)
}

func (m TrackStatusMessage) Type() controlMessageType {
	return messageTypeTrackStatus
}

func (m *TrackStatusMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = m.LargestLocation.append(buf)
	return m.Parameters.append(buf)
}

func (m *TrackStatusMessage) parse(v Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.StatusCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	n, err = m.LargestLocation.parse(v, data)
	if err != nil {
		return
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parseVersionSpecificParameters(data)
}
