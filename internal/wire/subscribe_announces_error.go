package wire

import "github.com/quic-go/quic-go/quicvarint"

// TODO: Add tests
type SubscribeAnnouncesErrorMessage struct {
	TrackNamespacePrefix Tuple
	ErrorCode            uint64
	ReasonPhrase         string
}

func (m SubscribeAnnouncesErrorMessage) Type() controlMessageType {
	return messageTypeSubscribeAnnouncesError
}

func (m *SubscribeAnnouncesErrorMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespacePrefix.append(buf)
	buf = quicvarint.Append(buf, m.ErrorCode)
	return appendVarIntBytes(buf, []byte(m.ReasonPhrase))
}

func (m *SubscribeAnnouncesErrorMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespacePrefix, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.ErrorCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	reasonPhrase, n, err := parseVarIntBytes(data)
	if err != nil {
		return err
	}
	m.ReasonPhrase = string(reasonPhrase)
	return nil
}
