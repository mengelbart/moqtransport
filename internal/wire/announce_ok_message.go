package wire

type AnnounceOkMessage struct {
	TrackNamespace Tuple
}

func (m AnnounceOkMessage) GetTrackNamespace() string {
	return m.TrackNamespace.String()
}

func (m AnnounceOkMessage) Type() controlMessageType {
	return messageTypeAnnounceOk
}

func (m *AnnounceOkMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return buf
}

func (m *AnnounceOkMessage) parse(data []byte) (err error) {
	m.TrackNamespace, _, err = parseTuple(data)
	return err
}
