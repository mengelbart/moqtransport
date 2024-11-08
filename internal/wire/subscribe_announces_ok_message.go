package wire

// TODO: Add tests
type SubscribeAnnouncesOkMessage struct {
	TrackNamespacePrefix Tuple
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
