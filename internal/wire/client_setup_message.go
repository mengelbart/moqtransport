package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type ClientSetupMessage struct {
	SupportedVersions versions
	SetupParameters   KVPList
}

func (m *ClientSetupMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("type", "client_setup"),
		slog.Uint64("number_of_supported_versions", uint64(len(m.SupportedVersions))),
		slog.Any("supported_versions", m.SupportedVersions),
		slog.Uint64("number_of_parameters", uint64(len(m.SetupParameters))),
	}
	if len(m.SetupParameters) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", m.SetupParameters),
		)
	}
	return slog.GroupValue(attrs...)
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

func (m *ClientSetupMessage) parse(_ Version, data []byte) error {
	n, err := m.SupportedVersions.parse(data)
	if err != nil {
		return err
	}
	data = data[n:]
	m.SetupParameters = KVPList{}
	return m.SetupParameters.parseNum(data)
}
