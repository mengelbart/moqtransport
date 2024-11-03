package wire

type GoAwayMessage struct {
	NewSessionURI string
}

func (m GoAwayMessage) Type() controlMessageType {
	return messageTypeGoAway
}

func (m *GoAwayMessage) Append(buf []byte) []byte {
	buf = appendVarIntBytes(buf, []byte(m.NewSessionURI))
	return buf
}

func (m *GoAwayMessage) parse(data []byte) (err error) {
	newSessionURI, _, err := parseVarIntBytes(data)
	m.NewSessionURI = string(newSessionURI)
	return err
}
