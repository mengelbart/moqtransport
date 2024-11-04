package wire

// TODO: Add tests
type SusbcribeAnnouncesOk struct {
	TrackNamespacePrefix Tuple
}

func (m SusbcribeAnnouncesOk) Type() controlMessageType {
	return messageTypeSubscribeAnnouncesOk
}

func (m *SusbcribeAnnouncesOk) Append(buf []byte) []byte {
	return m.TrackNamespacePrefix.append(buf)
}

func (m *SusbcribeAnnouncesOk) parse(data []byte) (err error) {
	m.TrackNamespacePrefix, _, err = parseTuple(data)
	return err
}
