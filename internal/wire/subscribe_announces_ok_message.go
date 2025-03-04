package wire

import (
	"log/slog"
)

var _ slog.LogValuer = (*SubscribeAnnouncesOkMessage)(nil)

// TODO: Add tests
type SubscribeAnnouncesOkMessage struct {
	TrackNamespacePrefix Tuple
}

func (m *SubscribeAnnouncesOkMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "subscribe_announces_ok"),
		slog.Any("track_namespace_prefix", m.TrackNamespacePrefix),
	)
}

func (m SubscribeAnnouncesOkMessage) Type() controlMessageType {
	return messageTypeSubscribeAnnouncesOk
}

func (m *SubscribeAnnouncesOkMessage) Append(buf []byte) []byte {
	return m.TrackNamespacePrefix.append(buf)
}

func (m *SubscribeAnnouncesOkMessage) parse(data []byte) (err error) {
	m.TrackNamespacePrefix, _, err = parseTuple(data)
	return err
}
