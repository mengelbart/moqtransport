package moqtransport

import (
	"io"
	"sync"

	"github.com/quic-go/quic-go"
)

type sendMode int

const (
	streamPerObject sendMode = iota
	streamPerGroup
	singleStream
	datagram
)

type ReliableObjectWriter struct {
	tid uint64
	gs  uint64
	os  uint64
	oso uint64
	w   io.Writer
}

func newReliableObjectWriter(t *SendTrack) (*ReliableObjectWriter, error) {
	if t.stream == nil {
		var err error
		t.stream, err = t.conn.OpenUniStream()
		if err != nil {
			return nil, err
		}
	}
	return &ReliableObjectWriter{
		tid: t.id,
		gs:  0,
		os:  0,
		oso: 0,
		w:   t.stream,
	}, nil
}

func (w *ReliableObjectWriter) Write(b []byte) (n int, err error) {
	om := &objectMessage{
		HasLength:       true,
		TrackID:         w.tid,
		GroupSequence:   w.gs,
		ObjectSequence:  w.os,
		ObjectSendOrder: w.oso,
		ObjectPayload:   b,
	}
	buf := make([]byte, 0, 1_024_000)
	buf = om.append(buf)
	return w.w.Write(buf)
}

type CancellableObjectWriter struct {
	w sendStream
}

func newCancellableObjectWriter(t *SendTrack) (*CancellableObjectWriter, error) {
	stream, err := t.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	om := &objectMessage{
		HasLength:       false,
		TrackID:         0,
		GroupSequence:   0,
		ObjectSequence:  0,
		ObjectSendOrder: 0,
	}
	buf := make([]byte, 0, 1_024_000)
	buf = om.append(buf)
	_, err = stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &CancellableObjectWriter{
		w: stream,
	}, nil
}

func (w *CancellableObjectWriter) Write(b []byte) (n int, err error) {
	return w.w.Write(b)
}

func (w *CancellableObjectWriter) Cancel() {
	if qs, ok := w.w.(quic.SendStream); ok {
		qs.CancelWrite(1)
	}
}

func (w *CancellableObjectWriter) Close() error {
	return w.w.Close()
}

type SendTrack struct {
	conn   connection
	mode   sendMode
	id     uint64
	stream sendStream
	lock   sync.Mutex
}

func newSendTrack(conn connection) *SendTrack {
	return &SendTrack{
		conn:   conn,
		mode:   singleStream,
		id:     0,
		stream: nil,
	}
}

func (t *SendTrack) StartReliableObject() (*ReliableObjectWriter, error) {
	return newReliableObjectWriter(t)
}

func (t *SendTrack) StartCancellableObject() (*CancellableObjectWriter, error) {
	return newCancellableObjectWriter(t)
}

func (t *SendTrack) SendNewUnreliableObject(b []byte) error {
	return t.conn.SendMessage(b)
}
