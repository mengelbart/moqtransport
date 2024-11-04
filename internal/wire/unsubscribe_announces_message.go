package wire

// TODO: Add tests
type UnsubscribeAnnouncesMessage struct {
	TrackNamespacePrefix Tuple
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
