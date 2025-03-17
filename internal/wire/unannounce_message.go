package wire

import (
	"log/slog"
)

type UnannounceMessage struct {
	TrackNamespace Tuple
}

func (m *UnannounceMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "unannounce"),
		slog.Any("track_namespace", m.TrackNamespace),
	)
}

func (m UnannounceMessage) Type() controlMessageType {
	return messageTypeUnannounce
}

func (m *UnannounceMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return buf
}

func (p *UnannounceMessage) parse(_ Version, data []byte) (err error) {
	p.TrackNamespace, _, err = parseTuple(data)
	return err
}
