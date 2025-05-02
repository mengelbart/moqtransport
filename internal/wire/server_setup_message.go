package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type ServerSetupMessage struct {
	SelectedVersion Version
	SetupParameters KVPList
}

func (m *ServerSetupMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "server_setup"),
		slog.Uint64("selected_version", uint64(m.SelectedVersion)),
		slog.Uint64("number_of_parameters", uint64(len(m.SetupParameters))),
	}
	if len(m.SetupParameters) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", m.SetupParameters),
		)
	}
	return slog.GroupValue(attrs...)
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

func (m *ServerSetupMessage) parse(_ Version, data []byte) error {
	sv, n, err := quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.SelectedVersion = Version(sv)
	m.SetupParameters = KVPList{}
	return m.SetupParameters.parseNum(data)
}
