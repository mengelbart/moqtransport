package webtransportmoq

import (
	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/webtransport-go"
)

var _ moqtransport.SendStream = (*SendStream)(nil)

type SendStream struct {
	stream webtransport.SendStream
}

// Write implements moqtransport.SendStream.
func (s *SendStream) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

// Reset implements moqtransport.SendStream
func (s *SendStream) Reset(code uint32) {
	s.stream.CancelWrite(webtransport.StreamErrorCode(code))
}

// Close implements moqtransport.SendStream.
func (s *SendStream) Close() error {
	return s.stream.Close()
}
