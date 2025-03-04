package wire

import (
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/quic-go/quic-go/quicvarint"
)

var _ slog.LogValuer = (*ServerSetupMessage)(nil)

type ServerSetupMessage struct {
	SelectedVersion Version
	SetupParameters Parameters
}

func (m *ServerSetupMessage) LogValue() slog.Value {
	sps := []Parameter{}
	for _, v := range m.SetupParameters {
		sps = append(sps, v)
	}
	attrs := []slog.Attr{
		slog.String("type", "server_setup"),
		slog.Uint64("selected_version", uint64(m.SelectedVersion)),
		slog.Uint64("number_of_parameters", uint64(len(sps))),
	}
	if len(sps) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", slices.Collect(slices.Map(sps, func(e Parameter) any { return e }))),
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
