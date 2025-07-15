package webtransportmoq

import (
	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/webtransport-go"
)

var _ moqtransport.Stream = (*Stream)(nil)

type Stream struct {
	stream *webtransport.Stream
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
	s.stream.CancelWrite(webtransport.StreamErrorCode(code))
}

// Stop implements moqtransport.Stream.
func (s *Stream) Stop(code uint32) {
	s.stream.CancelRead(webtransport.StreamErrorCode(code))
}

// StreamID implements moqtransport.Stream.
func (s *Stream) StreamID() uint64 {
	return uint64(s.stream.StreamID())
}
