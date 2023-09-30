package moqtransport

import "io"

type ReceiveTrack struct {
	receiveBuffer io.ReadWriter
}

func newReceiveTrack() *ReceiveTrack {
	return &ReceiveTrack{
		receiveBuffer: nil,
	}
}

func (t *ReceiveTrack) push(msg *ObjectMessage) error {
	_, err := t.receiveBuffer.Write(msg.ObjectPayload)
	if err != nil {
		return err
	}
	return nil
}

func (t *ReceiveTrack) Read(p []byte) (n int, err error) {
	return 0, nil
}
