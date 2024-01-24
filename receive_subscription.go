package moqtransport

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

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

func (s *ReceiveSubscription) readTrackHeaderStream(rs receiveStream) {
	parser := newParser(quicvarint.NewReader(rs))
	for {
		msg, err := parser.parseStreamHeaderTrackObject()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		if _, err = s.push(msg); err != nil {
			panic(err)
		}
	}
}

func (s *ReceiveSubscription) readGroupHeaderStream(rs receiveStream) {
	parser := newParser(quicvarint.NewReader(rs))
	for {
		msg, err := parser.parseStreamHeaderGroupObject()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		if _, err = s.push(msg); err != nil {
			panic(err)
		}
	}
}
