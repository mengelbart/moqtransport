package wire

import (
	"log/slog"
	"time"

	"github.com/quic-go/quic-go/quicvarint"
)

type SubscribeOkMessage struct {
	RequestID       uint64
	TrackAlias      uint64
	Expires         time.Duration
	GroupOrder      uint8
	ContentExists   bool
	LargestLocation Location
	Parameters      KVPList
}

func (m *SubscribeOkMessage) LogValue() slog.Value {
	ce := 0
	if m.ContentExists {
		ce = 1
	}
	attrs := []slog.Attr{
		slog.String("type", "subscribe_ok"),
		slog.Uint64("track_alias", m.TrackAlias),
		slog.Uint64("request_id", m.RequestID),
		slog.Uint64("expires", uint64(m.Expires.Milliseconds())),
		slog.Any("group_order", m.GroupOrder),
		slog.Int("content_exists", ce),
	}
	if m.ContentExists {
		attrs = append(attrs,
			slog.Uint64("largest_group_id", m.LargestLocation.Group),
			slog.Uint64("largest_object_id", m.LargestLocation.Object),
		)
	}
	attrs = append(attrs,
		slog.Uint64("number_of_parameters", uint64(len(m.Parameters))),
	)
	if len(m.Parameters) > 0 {
		attrs = append(attrs,
			slog.Any("subscribe_parameters", m.Parameters),
		)

	}
	return slog.GroupValue(attrs...)
}

func (m SubscribeOkMessage) Type() controlMessageType {
	return messageTypeSubscribeOk
}

func (m *SubscribeOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, uint64(m.Expires))
	buf = append(buf, m.GroupOrder)
	if m.ContentExists {
		buf = append(buf, 1) // ContentExists=true
		buf = m.LargestLocation.append(buf)
		return m.Parameters.appendNum(buf)
	}
	buf = append(buf, 0) // ContentExists=false
	return m.Parameters.appendNum(buf)
}

func (m *SubscribeOkMessage) parse(v Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	m.TrackAlias, n, err = quicvarint.Parse(data)
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
		m.Parameters = KVPList{}
		return m.Parameters.parseNum(data)
	}

	n, err = m.LargestLocation.parse(v, data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.Parameters = KVPList{}
	return m.Parameters.parseNum(data)
}
