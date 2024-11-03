package wire

type TrackStatusRequestMessage struct {
	TrackNamespace Tuple
	TrackName      string
}

func (m TrackStatusRequestMessage) Type() controlMessageType {
	return messageTypeTrackStatusRequest
}

func (m *TrackStatusRequestMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return appendVarIntBytes(buf, []byte(m.TrackName))
}

func (m *TrackStatusRequestMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return
	}
	data = data[n:]

	trackName, _, err := parseVarIntBytes(data)
	m.TrackName = string(trackName)
	return err
}
