package webtransportgo

import (
	"time"

	"github.com/mengelbart/moqtransport/quic"
	"github.com/quic-go/webtransport-go"
)

var _ quic.ReceiveStream = (*ReceiveStream)(nil)

type ReceiveStream struct {
	stream *webtransport.ReceiveStream
}

// Read implements moqtransport.ReceiveStream.
func (r *ReceiveStream) Read(p []byte) (n int, err error) {
	return r.stream.Read(p)
}

// Stop implements moqtransport.ReceiveStream. The deadline unblocks a Read
// that waits for the session to close after the peer reset the stream with
// WTSessionGoneErrorCode, which CancelRead alone does not.
func (r *ReceiveStream) Stop(code uint32) {
	r.stream.CancelRead(webtransport.StreamErrorCode(code))
	_ = r.stream.SetReadDeadline(time.Now())
}

// StreamID implements moqtransport.ReceiveStream
func (r *ReceiveStream) StreamID() uint64 {
	return uint64(r.stream.StreamID())
}
