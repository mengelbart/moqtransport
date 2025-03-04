package moqtransport

import (
	"context"
	"errors"
	"io"
)

// Protocol is a transport protocol supported by MoQ.
type Protocol int

// The supported protocols
const (
	ProtocolQUIC Protocol = iota
	ProtocolWebTransport
)

func (p Protocol) String() string {
	switch p {
	case ProtocolQUIC:
		return "quic"
	case ProtocolWebTransport:
		return "webtransport"
	default:
		return "invalid protocol"
	}
}

// Perspective indicates whether the connection is a client or a server
type Perspective int

// The perspectives
const (
	PerspectiveServer Perspective = iota
	PerspectiveClient
)

func (p Perspective) String() string {
	switch p {
	case PerspectiveServer:
		return "server"
	case PerspectiveClient:
		return "client"
	default:
		return "invalid perspective"
	}
}

// A Stream is the interface implemented by bidirectional streams.
type Stream interface {
	ReceiveStream
	SendStream
}

// A Stream is the interface implemented by the receiving end of unidirectional
// streams.
type ReceiveStream interface {
	// Read reads from the stream.
	io.Reader

	// Stop stops reading from the stream and sends a signal to the sender to
	// stop sending on the stream.
	Stop(uint32)

	// StreamID returns the ID of the stream
	StreamID() uint64
}

// A Stream is the interface implemented by the sending end of unidirectional
// streams.
type SendStream interface {
	// Write writes to the stream.
	// Close closes the stream and guarantees retransmissions until all data has
	// been received by the receiver or the stream is reset.
	io.WriteCloser

	// Reset closes the stream and stops retransmitting outstanding data.
	Reset(uint32)

	// StreamID returns the ID of the stream
	StreamID() uint64
}

var ErrDatagramSupportDisabled = errors.New("datagram support disabled")

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

	// Protocol returns the underlying Protocol of the connection.
	Protocol() Protocol

	// Perspective returns the perspective of the connection.
	Perspective() Perspective
}
