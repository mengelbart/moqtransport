package quicmoq

import (
	"errors"

	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/quic-go"
)

type stream struct {
	qs quic.Stream
}

func (s *stream) Read(p []byte) (int, error) {
	return s.qs.Read(p)
}

func (s *stream) Write(p []byte) (int, error) {
	return s.qs.Write(p)
}

func (s *stream) Close() error {
	return s.qs.Close()
}

func (s *stream) CancelRead(code uint64) {
	s.qs.CancelRead(quic.StreamErrorCode(code))
}

func (s *stream) CancelWrite(code uint64) {
	s.qs.CancelWrite(quic.StreamErrorCode(code))
}

type receiveStream struct {
	stream quic.ReceiveStream
}

func (s *receiveStream) Read(p []byte) (int, error) {
	n, err := s.stream.Read(p)
	if err != nil {
		var streamErr *quic.StreamError
		if errors.As(err, &streamErr) {
			if streamErr.Remote {
				return n, moqtransport.ApplicationError{
					Code:   0,
					Mesage: "stream reset by peer",
				}
			}
			return n, moqtransport.ApplicationError{
				Code:   0,
				Mesage: "stream reset",
			}
		}
		return n, err
	}
	return n, nil
}

func (s *receiveStream) CancelRead(code uint64) {
	s.stream.CancelRead(quic.StreamErrorCode(code))
}

type sendStream struct {
	stream quic.SendStream
}

func (s *sendStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *sendStream) Close() error {
	return s.stream.Close()
}

func (s *sendStream) CancelWrite(code uint64) {
	s.stream.CancelWrite(quic.StreamErrorCode(code))
}
