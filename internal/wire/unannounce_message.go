package wire

type UnannounceMessage struct {
	TrackNamespace Tuple
}

func (m UnannounceMessage) Type() controlMessageType {
	return messageTypeUnannounce
}

func (m *UnannounceMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	return buf
}

func (p *UnannounceMessage) parse(data []byte) (err error) {
	p.TrackNamespace, _, err = parseTuple(data)
	return err
}
