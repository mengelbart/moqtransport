package wire

import (
	"bufio"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	objectTypeDatagram       uint64 = 0x01
	objectTypeDatagramStatus uint64 = 0x02
)

type StreamType int

const (
	StreamTypeSubgroup StreamType = 0x04
	StreamTypeFetch    StreamType = 0x05
)

type ObjectStatus int

const (
	ObjectStatusNormal ObjectStatus = iota
	ObjectStatusObjectDoesNotExist
	ObjectStatusGroupDoesNotExist
	ObjectStatusEndOfGroup
	ObjectStatusEndOfTrack
)

type ObjectMessage struct {
	TrackAlias        uint64
	GroupID           uint64
	SubgroupID        uint64
	ObjectID          uint64
	PublisherPriority uint8
	ObjectStatus      ObjectStatus
	ObjectPayload     []byte
}

func (m *ObjectMessage) AppendDatagram(buf []byte) []byte {
	buf = quicvarint.Append(buf, objectTypeDatagram)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = append(buf, m.PublisherPriority)
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func (m *ObjectMessage) AppendDatagramStatus(buf []byte) []byte {
	buf = quicvarint.Append(buf, objectTypeDatagramStatus)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = append(buf, m.PublisherPriority)
	buf = quicvarint.Append(buf, uint64(m.ObjectStatus))
	return buf
}

func (m *ObjectMessage) AppendSubgroup(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	if len(m.ObjectPayload) == 0 {
		buf = quicvarint.Append(buf, uint64(m.ObjectStatus))
	} else {
		buf = append(buf, m.ObjectPayload...)
	}
	return buf
}

func (m *ObjectMessage) AppendFetch(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.SubgroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = append(buf, m.PublisherPriority)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	if len(m.ObjectPayload) == 0 {
		buf = quicvarint.Append(buf, uint64(m.ObjectStatus))
	} else {
		buf = append(buf, m.ObjectPayload...)
	}
	return buf
}

func (m *ObjectMessage) ParseDatagram(data []byte) (parsed int, err error) {
	typ, n, err := quicvarint.Parse(data)
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
		return parsed, io.ErrUnexpectedEOF
	}
	m.PublisherPriority = data[0]
	parsed += 1
	data = data[1:]

	if typ == objectTypeDatagramStatus {
		var status uint64
		status, n, err = quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return
		}
		m.ObjectStatus = ObjectStatus(status)
		return
	}

	m.ObjectPayload = make([]byte, len(data))
	n = copy(m.ObjectPayload, data)
	parsed += n
	return
}

func (m *ObjectMessage) readSubgroup(r io.Reader) (err error) {
	br := bufio.NewReader(r)
	m.ObjectID, err = quicvarint.Read(br)
	if err != nil {
		return
	}
	length, err := quicvarint.Read(br)
	if err != nil {
		return
	}

	if length == 0 {
		var status uint64
		status, err = quicvarint.Read(br)
		if err != nil {
			return
		}
		m.ObjectStatus = ObjectStatus(status)
		return
	}
	m.ObjectPayload = make([]byte, length)
	_, err = io.ReadFull(r, m.ObjectPayload)
	return
}

func (m *ObjectMessage) readFetch(r io.Reader) (err error) {
	br := bufio.NewReader(r)
	m.GroupID, err = quicvarint.Read(br)
	if err != nil {
		return
	}
	m.SubgroupID, err = quicvarint.Read(br)
	if err != nil {
		return
	}
	m.ObjectID, err = quicvarint.Read(br)
	if err != nil {
		return
	}
	m.PublisherPriority, err = br.ReadByte()
	if err != nil {
		return
	}
	length, err := quicvarint.Read(br)
	if err != nil {
		return
	}

	if length == 0 {
		var status uint64
		status, err = quicvarint.Read(br)
		if err != nil {
			return
		}
		m.ObjectStatus = ObjectStatus(status)
		return
	}

	m.ObjectPayload = make([]byte, length)
	_, err = io.ReadFull(r, m.ObjectPayload)
	return
}
