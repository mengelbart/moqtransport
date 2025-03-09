package wire

import (
	"log/slog"

	"github.com/mengelbart/qlog"
)

type TrackStatusRequestMessage struct {
	TrackNamespace Tuple
	TrackName      string
}

func (m *TrackStatusRequestMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "track_status_request"),
		slog.Any("track_namespace", m.TrackNamespace),
		slog.Any("track_name", qlog.RawInfo{
			Length:        uint64(len(m.TrackName)),
			PayloadLength: uint64(len(m.TrackName)),
			Data:          []byte(m.TrackName),
		}),
	)
}

func (m TrackStatusRequestMessage) Type() controlMessageType {
	return messageTypeTrackStatusRequest
}

func (m *TrackStatusRequestMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return appendVarIntBytes(buf, []byte(m.TrackName))
}

func (m *TrackStatusRequestMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return
	}
	data = data[n:]

	trackName, _, err := parseVarIntBytes(data)
	m.TrackName = string(trackName)
	return err
}
