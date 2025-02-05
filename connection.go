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
	Stop(uint32)
}

type SendStream interface {
	io.WriteCloser
	Reset(uint32)
}

// Connection is the interface of a QUIC/WebTransport connection. New Transports
// expect an implementation of this interface as the underlying connection.
// Implementations based on quic-go and webtransport-go are provided in quicmoq
// and webTransportmoq.
type Connection interface {
	// AcceptStream returns the next stream opened by the peer, blocking until
	// one is available.
	AcceptStream(context.Context) (Stream, error)

	// AcceptUniStream returns the next unidirectional stream opened by the
	// peer, blocking until one is available.
	AcceptUniStream(context.Context) (ReceiveStream, error)

	// OpenStream opens a new bidirectional stream.
	OpenStream() (Stream, error)

	// OpenStreamSync opens a new bidirectional stream, blocking until it can be
	// opened.
	OpenStreamSync(context.Context) (Stream, error)

	// OpenUniStream opens a new unidirectional stream.
	OpenUniStream() (SendStream, error)

	// OpenUniStream opens a new unidirectional stream, blocking until it can be
	// opened.
	OpenUniStreamSync(context.Context) (SendStream, error)

	// SendDatagram sends a datagram.
	SendDatagram([]byte) error

	// ReceiveDatagram receives the next datagram, blocking until one is
	// available.
	ReceiveDatagram(context.Context) ([]byte, error)

	// CloseWithError closes the connection with an error code and a reason
	// string.
	CloseWithError(uint64, string) error

	// Context returns a context that will be cancelled when the connection is
	// closed.
	Context() context.Context
}
