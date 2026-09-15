package quicgo

import (
	"github.com/mengelbart/moqtransport/quic"
	quicgo "github.com/quic-go/quic-go"
)

var _ quic.Stream = (*Stream)(nil)

type Stream struct {
	stream *quicgo.Stream
}

// Read implements moqtransport.Stream.
func (s *Stream) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

// Write implements moqtransport.Stream.
func (s *Stream) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

// Close implements moqtransport.Stream.
func (s *Stream) Close() error {
	return s.stream.Close()
}

// Reset implements moqtransport.Stream.
func (s *Stream) Reset(code uint32) {
	s.stream.CancelWrite(quicgo.StreamErrorCode(code))
}

// Stop implements moqtransport.Stream.
func (s *Stream) Stop(code uint32) {
	s.stream.CancelRead(quicgo.StreamErrorCode(code))
}

// StreamID implements moqtransport.Stream.
func (s *Stream) StreamID() uint64 {
	return uint64(s.stream.StreamID())
}
