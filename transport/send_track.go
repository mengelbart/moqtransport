package transport

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
	conn Connection
	mode sendMode
}

func newSendTrack(conn Connection) *SendTrack {
	return &SendTrack{
		conn: conn,
	}
}

func (t *SendTrack) writeNewStream(b []byte) (int, error) {
	s, err := t.conn.OpenUniStream()
	if err != nil {
		return 0, err
	}
	return s.Write(b)
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
