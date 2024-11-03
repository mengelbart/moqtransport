package wire

type AnnounceCancelMessage struct {
	TrackNamespace Tuple
}

func (m AnnounceCancelMessage) GetTrackNamespace() string {
	return m.TrackNamespace.String()
}

func (m AnnounceCancelMessage) Type() controlMessageType {
	return messageTypeAnnounce
}

func (m *AnnounceCancelMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return buf
}

func (m *AnnounceCancelMessage) parse(data []byte) (err error) {
	m.TrackNamespace, _, err = parseTuple(data)
	return err
}
