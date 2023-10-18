package moqtransport

import (
	"context"
	"io"
)

// TODO: Streams must be wrapped properly for quic and webtransport The
// interfaces need to add CancelRead and CancelWrite for STOP_SENDING and
// RESET_STREAM purposes. The interface should allow implementations for quic
// and webtransport.
type stream interface {
	receiveStream
	sendStream
}

type receiveStream interface {
	io.Reader
}

type sendStream interface {
	io.WriteCloser
}

type connection interface {
	OpenStream() (stream, error)
	OpenStreamSync(context.Context) (stream, error)
	OpenUniStream() (sendStream, error)
	OpenUniStreamSync(context.Context) (sendStream, error)
	AcceptStream(context.Context) (stream, error)
	AcceptUniStream(context.Context) (receiveStream, error)
	ReceiveMessage(context.Context) ([]byte, error)
	CloseWithError(uint64, string) error
}
