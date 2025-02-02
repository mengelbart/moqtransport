package webtransportmoq

import "github.com/mengelbart/moqtransport"

var _ moqtransport.Stream = (*Stream)(nil)

type Stream struct {
	send    *SendStream
	receive *ReceiveStream
}

// Read implements moqtransport.Stream.
func (s *Stream) Read(p []byte) (n int, err error) {
	return s.receive.Read(p)
}

// Write implements moqtransport.Stream.
func (s *Stream) Write(p []byte) (n int, err error) {
	return s.send.Write(p)
}

// Close implements moqtransport.Stream.
func (s *Stream) Close() error {
	return s.send.Close()
}

// Reset implements moqtransport.Stream.
func (s *Stream) Reset(code uint32) {
	s.send.Reset(code)
}

// Stop implements moqtransport.Stream.
func (s *Stream) Stop(code uint32) {
	s.receive.Stop(code)
}
