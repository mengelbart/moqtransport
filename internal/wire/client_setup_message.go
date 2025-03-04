package wire

import (
	"log/slog"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/quic-go/quic-go/quicvarint"
)

var _ slog.LogValuer = (*ClientSetupMessage)(nil)

type ClientSetupMessage struct {
	SupportedVersions versions
	SetupParameters   Parameters
}

func (m *ClientSetupMessage) LogValue() slog.Value {
	sps := []Parameter{}
	for _, v := range m.SetupParameters {
		sps = append(sps, v)
	}
	attrs := []slog.Attr{
		slog.String("type", "client_setup"),
		slog.Uint64("number_of_supported_versions", uint64(len(m.SupportedVersions))),
		slog.Any("supported_versions", m.SupportedVersions),
		slog.Uint64("number_of_parameters", uint64(len(sps))),
	}
	if len(sps) > 0 {
		attrs = append(attrs,
			slog.Any("setup_parameters", slices.Collect(slices.Map(sps, func(e Parameter) any { return e }))),
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

func (m *ClientSetupMessage) parse(data []byte) error {
	n, err := m.SupportedVersions.parse(data)
	if err != nil {
		return err
	}
	data = data[n:]
	m.SetupParameters = Parameters{}
	return m.SetupParameters.parse(data, setupParameterTypes)
}
