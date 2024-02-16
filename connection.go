package moqtransport

import (
	"context"
	"io"
)

type Stream interface {
	ReceiveStream
	SendStream
}

type ReceiveStream interface {
	io.Reader
}

type SendStream interface {
	io.WriteCloser
}

type Connection interface {
	OpenStream() (Stream, error)
	OpenStreamSync(context.Context) (Stream, error)
	OpenUniStream() (SendStream, error)
	OpenUniStreamSync(context.Context) (SendStream, error)
	AcceptStream(context.Context) (Stream, error)
	AcceptUniStream(context.Context) (ReceiveStream, error)
	SendDatagram([]byte) error
	ReceiveDatagram(context.Context) ([]byte, error)
	CloseWithError(uint64, string) error
}
