package wire

import (
	"log/slog"

	"github.com/mengelbart/qlog"
	"github.com/quic-go/quic-go/quicvarint"
)

type TrackStatusRequestMessage struct {
	RequestID      uint64
	TrackNamespace Tuple
	TrackName      []byte
	Parameters     KVPList
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
	return messageTypeTrackStatus
}

func (m *TrackStatusRequestMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, []byte(m.TrackName))
	return m.Parameters.appendNum(buf)
}

func (m *TrackStatusRequestMessage) parse(_ Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.TrackName, n, err = parseVarIntBytes(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = KVPList{}
	return m.Parameters.parseNum(data)
}
