package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

type ServerSetupMessage struct {
	SelectedVersion Version
	SetupParameters Parameters
}

func (m ServerSetupMessage) Type() controlMessageType {
	return messageTypeServerSetup
}

func (m *ServerSetupMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(m.SelectedVersion))
	buf = quicvarint.Append(buf, uint64(len(m.SetupParameters)))
	for _, p := range m.SetupParameters {
		buf = p.append(buf)
	}
	return buf
}

func (m *ServerSetupMessage) parse(data []byte) error {
	sv, n, err := quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.SelectedVersion = Version(sv)
	m.SetupParameters = Parameters{}
	return m.SetupParameters.parse(data, setupParameterTypes)
}
