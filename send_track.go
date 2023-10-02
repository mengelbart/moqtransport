package moqtransport

import "errors"

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
	conn connection
	mode sendMode
	id   uint64
}

func newSendTrack(conn connection) *SendTrack {
	return &SendTrack{
		conn: conn,
	}
}

func (t *SendTrack) writeNewStream(b []byte) (int, error) {
	s, err := t.conn.OpenUniStream()
	if err != nil {
		return 0, err
	}
	om := &objectMessage{
		trackID:         t.id,
		groupSequence:   0,
		objectSequence:  0,
		objectSendOrder: 0,
		objectPayload:   b,
	}
	buf := make([]byte, 0, 64_000)
	buf = om.append(buf)
	return s.Write(buf)
}

func (t *SendTrack) Write(b []byte) (n int, err error) {
	switch t.mode {
	case streamPerObject:
		return t.writeNewStream(b)
	case streamPerGroup:
		// ...
		return 0, nil
	}
	return 0, errInvalidSendMode
}
