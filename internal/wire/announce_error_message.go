package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type AnnounceErrorMessage struct {
	TrackNamespace string
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m AnnounceErrorMessage) GetTrackNamespace() string {
	return m.TrackNamespace
}

func (m *AnnounceErrorMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceErrorMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	return buf
}

func (m *AnnounceErrorMessage) parse(reader messageReader) (err error) {
	m.TrackNamespace, err = parseVarIntString(reader)
	if err != nil {
		return err
	}
	m.ErrorCode, err = quicvarint.Read(reader)
	if err != nil {
		return err
	}
	m.ReasonPhrase, err = parseVarIntString(reader)
	return
}
