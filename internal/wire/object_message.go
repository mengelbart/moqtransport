package wire

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type ObjectStatus uint64

const (
	ObjectStatusNormal ObjectStatus = iota
	ObjectStatusObjectDoesNotExist
	ObjectStatusGroupDoesNotExist
	ObjectStatusEndOfGroup
	ObjectStatusEndOfTrack
)

type ObjectMessage struct {
	Type              ObjectMessageType
	SubscribeID       uint64
	TrackAlias        uint64
	GroupID           uint64
	ObjectID          uint64
	PublisherPriority uint8
	ObjectStatus      ObjectStatus
	ObjectPayload     []byte
}

func (m *ObjectMessage) Append(buf []byte) []byte {
	if m.Type == ObjectDatagramMessageType {
		buf = quicvarint.Append(buf, uint64(ObjectDatagramMessageType))
	} else {
		buf = quicvarint.Append(buf, uint64(ObjectStreamMessageType))
	}
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = append(buf, m.PublisherPriority)
	buf = quicvarint.Append(buf, uint64(m.ObjectStatus))
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func (m *ObjectMessage) parse(data []byte) (parsed int, err error) {
	var n int
	m.SubscribeID, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]
	m.TrackAlias, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]
	m.GroupID, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]
	m.ObjectID, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]
	if len(data) == 0 {
		return parsed, io.EOF
	}
	m.PublisherPriority = data[0]
	parsed += 1
	data = data[1:]
	var status uint64
	status, n, err = quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	m.ObjectStatus = ObjectStatus(status)
	data = data[n:]
	// TODO: make the message type an io.Reader and let the user read?
	m.ObjectPayload = make([]byte, len(data))
	n = copy(m.ObjectPayload, data)
	parsed += n
	return
}
