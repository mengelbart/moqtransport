package wire

import (
	"log/slog"
)

var _ slog.LogValuer = (*UnsubscribeAnnouncesMessage)(nil)

// TODO: Add tests
type UnsubscribeAnnouncesMessage struct {
	TrackNamespacePrefix Tuple
}

func (m *UnsubscribeAnnouncesMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "unsubscribe_announces"),
		slog.Any("track_namespace_prefix", m.TrackNamespacePrefix),
	)
}

func (m UnsubscribeAnnouncesMessage) Type() controlMessageType {
	return messageTypeUnsubscribeAnnounces
}

func (m *UnsubscribeAnnouncesMessage) Append(buf []byte) []byte {
	return m.TrackNamespacePrefix.append(buf)
}

func (m *UnsubscribeAnnouncesMessage) parse(data []byte) (err error) {
	m.TrackNamespacePrefix, _, err = parseTuple(data)
	return err
}
