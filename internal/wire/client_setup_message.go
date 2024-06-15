package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type ClientSetupMessage struct {
	SupportedVersions versions
	SetupParameters   Parameters
}

func (m *ClientSetupMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(clientSetupMessageType))
	buf = quicvarint.Append(buf, uint64(len(m.SupportedVersions)))
	for _, v := range m.SupportedVersions {
		buf = quicvarint.Append(buf, uint64(v))
	}
	buf = quicvarint.Append(buf, uint64(len(m.SetupParameters)))
	for _, p := range m.SetupParameters {
		buf = p.append(buf)
	}
	return buf
}

func (m *ClientSetupMessage) parse(reader messageReader) error {
	err := m.SupportedVersions.parse(reader)
	if err != nil {
		return err
	}
	m.SetupParameters = Parameters{}
	return m.SetupParameters.parse(reader)
}
