package quicmoq

import (
	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/quic-go"
)

var _ moqtransport.ReceiveStream = (*ReceiveStream)(nil)

type ReceiveStream struct {
	stream *quic.ReceiveStream
}

// Read implements moqtransport.ReceiveStream.
func (r *ReceiveStream) Read(p []byte) (n int, err error) {
	return r.stream.Read(p)
}

// Stop implements moqtransport.ReceiveStream.
func (r *ReceiveStream) Stop(code uint32) {
	r.stream.CancelRead(quic.StreamErrorCode(code))
}

// StreamID implements moqtransport.ReceiveStream
func (r *ReceiveStream) StreamID() uint64 {
	return uint64(r.stream.StreamID())
}
