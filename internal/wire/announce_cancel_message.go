package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceCancelMessage struct {
	TrackNamespace Tuple
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m *AnnounceCancelMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "announce_cancel"),
		slog.Any("track_namespace", m.TrackNamespace),
		slog.Uint64("error_code", m.ErrorCode),
		slog.String("reason", m.ReasonPhrase),
	)
}

func (m AnnounceCancelMessage) GetTrackNamespace() string {
	return m.TrackNamespace.String()
}

func (m AnnounceCancelMessage) Type() controlMessageType {
	return messageTypeAnnounce
}

func (m *AnnounceCancelMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	return buf
}

func (m *AnnounceCancelMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.ErrorCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	reasonPhrase, _, err := parseVarIntBytes(data)
	m.ReasonPhrase = string(reasonPhrase)
	return err
}
