package wire

import (
	"github.com/quic-go/quic-go/quicvarint"
)

// TODO: Add tests
type FetchOkMessage struct {
	SubscribeID         uint64
	GroupOrder          uint8
	EndOfTrack          uint8
	LargestGroupID      uint64
	LargestObjectID     uint64
	SubscribeParameters Parameters
}

func (m FetchOkMessage) Type() controlMessageType {
	return messageTypeFetchOk
}

func (m *FetchOkMessage) Append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = append(buf, m.GroupOrder)
	buf = append(buf, m.EndOfTrack)
	buf = quicvarint.Append(buf, m.LargestGroupID)
	buf = quicvarint.Append(buf, m.LargestObjectID)
	return m.SubscribeParameters.append(buf)
}

func (m *FetchOkMessage) parse(data []byte) (err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < 2 {
		return errLengthMismatch
	}
	m.GroupOrder = data[0]
	if m.GroupOrder > 2 {
		return errInvalidGroupOrder
	}
	m.EndOfTrack = data[1]
	data = data[2:]

	m.LargestGroupID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.LargestObjectID, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	return m.SubscribeParameters.parse(data, versionSpecificParameterTypes)
}
