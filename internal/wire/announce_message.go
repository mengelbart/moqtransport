package wire

type AnnounceMessage struct {
	TrackNamespace Tuple
	Parameters     Parameters
}

func (m AnnounceMessage) GetTrackNamespace() string {
	return m.TrackNamespace.String()
}

func (m AnnounceMessage) Type() controlMessageType {
	return messageTypeAnnounce
}

func (m *AnnounceMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return m.Parameters.append(buf)
}

func (m *AnnounceMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data)
}
