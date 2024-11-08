package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type ClientSetupMessage struct {
	SupportedVersions versions
	SetupParameters   Parameters
}

func (m ClientSetupMessage) Type() controlMessageType {
	return messageTypeClientSetup
}

func (m *ClientSetupMessage) Append(buf []byte) []byte {
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

func (m *ClientSetupMessage) parse(data []byte) error {
	n, err := m.SupportedVersions.parse(data)
	if err != nil {
		return err
	}
	data = data[n:]
	m.SetupParameters = Parameters{}
	return m.SetupParameters.parse(data, setupParameterTypes)
}
