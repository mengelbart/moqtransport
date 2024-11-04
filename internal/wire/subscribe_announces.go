package wire

// TODO: Add tests
type SubscribeAnnouncesMessage struct {
	TrackNamespacePrefix Tuple
	Parameters           Parameters
}

func (m SubscribeAnnouncesMessage) Type() controlMessageType {
	return messageTypeSubscribeAnnounces
}

func (m *SubscribeAnnouncesMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespacePrefix.append(buf)
	return m.Parameters.append(buf)
}

func (m *SubscribeAnnouncesMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespacePrefix, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data)
}
