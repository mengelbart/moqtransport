package moqtransport

import "io"

type ReceiveTrack struct {
	receiveBuffer io.Writer
	bufferReader  io.Reader
}

func newReceiveTrack() *ReceiveTrack {
	r, w := io.Pipe()
	return &ReceiveTrack{
		receiveBuffer: w,
		bufferReader:  r,
	}
}

func (t *ReceiveTrack) push(msg *objectMessage) error {
	_, err := t.receiveBuffer.Write(msg.ObjectPayload)
	if err != nil {
		return err
	}
	return nil
}

func (t *ReceiveTrack) Read(p []byte) (n int, err error) {
	return t.bufferReader.Read(p)
}
