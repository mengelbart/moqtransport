package quicmoq

import (
	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/quic-go"
)

var _ moqtransport.SendStream = (*SendStream)(nil)

type SendStream struct {
	stream quic.SendStream
}

// Write implements moqtransport.SendStream.
func (s *SendStream) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

// Reset implements moqtransport.SendStream
func (s *SendStream) Reset(code uint32) {
	s.stream.CancelWrite(quic.StreamErrorCode(code))
}

// Close implements moqtransport.SendStream.
func (s *SendStream) Close() error {
	return s.stream.Close()
}

// StreamID implements moqtransport.SendStream
func (s *SendStream) StreamID() uint64 {
	return uint64(s.stream.StreamID())
}
