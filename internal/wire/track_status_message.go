package wire

import (
	"log/slog"

	"github.com/mengelbart/qlog"
	"github.com/quic-go/quic-go/quicvarint"
)

type TrackStatusMessage struct {
	TrackNamespace Tuple
	TrackName      string
	StatusCode     uint64
	LastGroupID    uint64
	LastObjectID   uint64
}

func (m *TrackStatusMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "track_status"),
		slog.Any("track_namespace", m.TrackNamespace),
		slog.Any("track_name", qlog.RawInfo{
			Length:        uint64(len(m.TrackName)),
			PayloadLength: uint64(len(m.TrackName)),
			Data:          []byte(m.TrackName),
		}),
		slog.Uint64("status_code", m.StatusCode),
		slog.Uint64("last_group_id", m.LastGroupID),
		slog.Uint64("last_object_id", m.LastObjectID),
	)
}

func (m TrackStatusMessage) Type() controlMessageType {
	return messageTypeTrackStatus
}

func (m *TrackStatusMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, []byte(m.TrackName))
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = quicvarint.Append(buf, m.LastGroupID)
	return quicvarint.Append(buf, m.LastObjectID)
}

func (m *TrackStatusMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return
	}
	data = data[n:]

	trackName, n, err := parseVarIntBytes(data)
	if err != nil {
		return
	}
	m.TrackName = string(trackName)
	data = data[n:]

	m.StatusCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LastGroupID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LastObjectID, _, err = quicvarint.Parse(data)
	return err
}
