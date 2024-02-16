package xnetquic

import (
	"github.com/mengelbart/moqtransport"
	"golang.org/x/net/quic"
)

var _ moqtransport.ReceiveStream = (*Stream)(nil)
var _ moqtransport.SendStream = (*Stream)(nil)
var _ moqtransport.Stream = (*Stream)(nil)

type Stream struct {
	stream *quic.Stream
}

// Read implements moqtransport.ReceiveStream.
func (s *Stream) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

// Stop implements moqtransport.ReceiveStream.
func (s *Stream) Stop(code uint32) {
	s.stream.CloseRead()
}

// Write implements moqtransport.SendStream.
func (s *Stream) Write(p []byte) (int, error) {
	n, err := s.stream.Write(p)
	if err != nil {
		return n, err
	}
	s.stream.Flush()
	return n, nil
}

// Close implements moqtransport.SendStream.
func (s *Stream) Close() error {
	s.stream.CloseWrite()
	return nil
}

// Reset implements moqtransport.SendStream.
func (s *Stream) Reset(code uint32) {
	s.stream.Reset(uint64(code))
}
