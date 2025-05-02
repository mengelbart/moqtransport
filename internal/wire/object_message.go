package wire

import (
	"bufio"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type StreamType uint64

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
	TrackAlias             uint64
	GroupID                uint64
	SubgroupID             uint64
	ObjectID               uint64
	PublisherPriority      uint8
	ObjectExtensionHeaders KVPList
	ObjectStatus           ObjectStatus
	ObjectPayload          []byte
}

func (m *ObjectMessage) AppendSubgroup(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = m.ObjectExtensionHeaders.appendLength(buf)
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
	buf = m.ObjectExtensionHeaders.appendLength(buf)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	if len(m.ObjectPayload) == 0 {
		buf = quicvarint.Append(buf, uint64(m.ObjectStatus))
	} else {
		buf = append(buf, m.ObjectPayload...)
	}
	return buf
}

func (m *ObjectMessage) readSubgroup(r io.Reader) (err error) {
	br := bufio.NewReader(r)
	m.ObjectID, err = quicvarint.Read(br)
	if err != nil {
		return
	}

	if m.ObjectExtensionHeaders == nil {
		m.ObjectExtensionHeaders = KVPList{}
	}
	if err = m.ObjectExtensionHeaders.parseLengthReader(br); err != nil {
		return err
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
	if m.ObjectExtensionHeaders == nil {
		m.ObjectExtensionHeaders = KVPList{}
	}
	if err = m.ObjectExtensionHeaders.parseLengthReader(br); err != nil {
		return err
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
