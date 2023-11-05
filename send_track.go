package moqtransport

import (
	"errors"
)

var (
	errInvalidSendMode = errors.New("invalid send mode")
)

type sendMode int

const (
	streamPerObject sendMode = iota
	streamPerGroup
	singleStream
	datagram
)

type SendTrack struct {
	conn   connection
	mode   sendMode
	id     uint64
	stream sendStream
}

func newSendTrack(conn connection) *SendTrack {
	s, err := conn.OpenUniStream()
	if err != nil {
		// TODO
		panic(err)
	}
	return &SendTrack{
		conn:   conn,
		mode:   singleStream,
		id:     0,
		stream: s,
	}
}

func (t *SendTrack) writeNewStream(b []byte) (int, error) {
	s, err := t.conn.OpenUniStream()
	if err != nil {
		return 0, err
	}
	om := &objectMessage{
		hasLength:       false,
		trackID:         t.id,
		groupSequence:   0,
		objectSequence:  0,
		objectSendOrder: 0,
		objectPayload:   b,
	}
	buf := make([]byte, 0, 64_000)
	buf = om.append(buf)
	defer s.Close()
	return s.Write(buf)
}

func (t *SendTrack) Write(b []byte) (n int, err error) {
	switch t.mode {
	case streamPerObject:
		return t.writeNewStream(b)
	case streamPerGroup:
		// ...
	case singleStream:
		om := &objectMessage{
			hasLength:       true,
			trackID:         t.id,
			groupSequence:   0,
			objectSequence:  0,
			objectSendOrder: 0,
			objectPayload:   b,
		}
		buf := make([]byte, 0, 64_000)
		buf = om.append(buf)
		return t.stream.Write(buf)
	}
	return 0, errInvalidSendMode
}
