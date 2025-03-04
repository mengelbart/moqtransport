package wire

import (
	"log/slog"
	"time"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/quic-go/quic-go/quicvarint"
)

var _ slog.LogValuer = (*SubscribeOkMessage)(nil)

type SubscribeOkMessage struct {
	SubscribeID     uint64
	Expires         time.Duration
	GroupOrder      uint8
	ContentExists   bool
	LargestGroupID  uint64
	LargestObjectID uint64
	Parameters      Parameters
}

func (m *SubscribeOkMessage) LogValue() slog.Value {
	sps := []Parameter{}
	for _, v := range m.Parameters {
		sps = append(sps, v)
	}
	attrs := []slog.Attr{
		slog.String("type", "subscribe_ok"),
		slog.Uint64("subscribe_id", m.SubscribeID),
		slog.Uint64("expires", uint64(m.Expires.Milliseconds())),
		slog.Any("group_order", m.GroupOrder),
		slog.Any("content_exists", m.ContentExists),
	}
	if m.ContentExists {
		attrs = append(attrs,
			slog.Uint64("largest_group_id", m.LargestGroupID),
			slog.Uint64("largest_object_id", m.LargestObjectID),
		)
	}
	attrs = append(attrs,
		slog.Uint64("number_of_parameters", uint64(len(sps))),
	)
	if len(sps) > 0 {
		attrs = append(attrs,
			slog.Any("subscribe_parameters", slices.Collect(slices.Map(sps, func(e Parameter) any { return e }))),
		)

	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeOkMessage) GetSubscribeID() uint64 {
	return m.SubscribeID
}

func (m SubscribeOkMessage) Type() controlMessageType {
	return messageTypeSubscribeOk
}

func (m *SubscribeOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, uint64(m.Expires))
	buf = append(buf, m.GroupOrder)
	if m.ContentExists {
		buf = append(buf, 1) // ContentExists=true
		buf = quicvarint.Append(buf, m.LargestGroupID)
		buf = quicvarint.Append(buf, m.LargestObjectID)
		return m.Parameters.append(buf)
	}
	buf = append(buf, 0) // ContentExists=false
	return m.Parameters.append(buf)
}

func (m *SubscribeOkMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	expires, n, err := quicvarint.Parse(data)
	if err != nil {
		return
	}
	m.Expires = time.Duration(expires) * time.Millisecond
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.GroupOrder = data[0]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	if data[1] != 0 && data[1] != 1 {
		return errInvalidContentExistsByte
	}
	m.ContentExists = data[1] == 1
	data = data[2:]

	if !m.ContentExists {
		m.Parameters = Parameters{}
		return m.Parameters.parse(data, versionSpecificParameterTypes)
	}

	m.LargestGroupID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.LargestObjectID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.Parameters = Parameters{}
	return m.Parameters.parse(data, versionSpecificParameterTypes)
}
