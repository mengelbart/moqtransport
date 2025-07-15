package wire

import (
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type PublishMessage struct {
	RequestID       uint64
	TrackNamespace  Tuple
	TrackName       []byte
	TrackAlias      uint64
	GroupOrder      uint8
	ContentExists   uint8
	LargestLocation Location
	Forward         uint8
	Parameters      KVPList
}

func (m *PublishMessage) LogValue() slog.Value {
	return slog.GroupValue()
}

func (m *PublishMessage) Type() controlMessageType {
	return messageTypePublish
}

func (m *PublishMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.RequestID)
	buf = m.TrackNamespace.append(buf)
	buf = appendVarIntBytes(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = append(buf, m.GroupOrder)
	buf = append(buf, m.ContentExists)
	if m.ContentExists > 0 {
		buf = m.LargestLocation.append(buf)
	}
	buf = append(buf, m.Forward)
	return m.Parameters.appendNum(buf)
}

func (m *PublishMessage) parse(v Version, data []byte) (err error) {
	var n int
	m.RequestID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackNamespace, n, err = parseTuple(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackName, n, err = parseVarIntBytes(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.TrackAlias, n, err = quicvarint.Parse(data)
	if err != nil {
		return
	}
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.GroupOrder = data[0]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	m.ContentExists = data[1]
	if m.ContentExists > 1 {
		return errInvalidContentExistsByte
	}
	data = data[2:]

	if m.ContentExists == 1 {
		n, err = m.LargestLocation.parse(v, data)
		if err != nil {
			return err
		}
		data = data[n:]
	}

	if len(data) < 1 {
		return errLengthMismatch
	}
	m.Forward = data[1]
	if m.Forward > 1 {
		return errInvalidForwardFlag
	}
	m.Parameters = KVPList{}
	return m.Parameters.parseNum(data)
}
