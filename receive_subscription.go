package moqtransport

import "io"

type ReceiveSubscription struct {
	responseCh chan subscribeIDer

	writeBuffer io.Writer
	readBuffer  io.Reader
}

func newReceiveSubscription() *ReceiveSubscription {
	r, w := io.Pipe()
	return &ReceiveSubscription{
		responseCh:  make(chan subscribeIDer),
		writeBuffer: w,
		readBuffer:  r,
	}
}

type payloader interface {
	payload() []byte
}

func (s *ReceiveSubscription) push(p payloader) (int, error) {
	return s.writeBuffer.Write(p.payload())
}

func (s *ReceiveSubscription) Read(buf []byte) (int, error) {
	return s.readBuffer.Read(buf)
}
