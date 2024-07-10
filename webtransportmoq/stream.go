package webtransportmoq

import (
	"errors"

	"github.com/mengelbart/moqtransport"
	"github.com/quic-go/webtransport-go"
)

type stream struct {
	qs webtransport.Stream
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
	s.qs.CancelRead(webtransport.StreamErrorCode(code))
}

func (s *stream) CancelWrite(code uint64) {
	s.qs.CancelWrite(webtransport.StreamErrorCode(code))
}

type receiveStream struct {
	stream webtransport.ReceiveStream
}

func (s *receiveStream) Read(p []byte) (int, error) {
	n, err := s.stream.Read(p)
	if err != nil {
		var streamErr *webtransport.StreamError
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
	s.stream.CancelRead(webtransport.StreamErrorCode(code))
}

type sendStream struct {
	stream webtransport.SendStream
}

func (s *sendStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *sendStream) Close() error {
	return s.stream.Close()
}

func (s *sendStream) CancelWrite(code uint64) {
	s.stream.CancelWrite(webtransport.StreamErrorCode(code))
}
