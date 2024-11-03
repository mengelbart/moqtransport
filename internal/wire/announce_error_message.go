package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceErrorMessage struct {
	TrackNamespace Tuple
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m AnnounceErrorMessage) GetTrackNamespace() string {
	return m.TrackNamespace.String()
}

func (m AnnounceErrorMessage) Type() controlMessageType {
	return messageTypeAnnounceError
}

func (m *AnnounceErrorMessage) Append(buf []byte) []byte {
	buf = m.TrackNamespace.append(buf)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntBytes(buf, []byte(m.ReasonPhrase))
	return buf
}

func (m *AnnounceErrorMessage) parse(data []byte) (err error) {
	var n int
	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.ErrorCode, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	reasonPhrase, _, err := parseVarIntBytes(data)
	m.ReasonPhrase = string(reasonPhrase)
	return err
}
