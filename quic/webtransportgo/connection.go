package webtransportgo

import (
	"context"

	"github.com/mengelbart/moqtransport/quic"
	"github.com/quic-go/webtransport-go"
)

type webTransportConn struct {
	session     *webtransport.Session
	perspective quic.Perspective
}

func NewServer(conn *webtransport.Session) quic.Connection {
	return New(conn, quic.PerspectiveServer)
}

func NewClient(conn *webtransport.Session) quic.Connection {
	return New(conn, quic.PerspectiveClient)
}

func New(session *webtransport.Session, perspective quic.Perspective) quic.Connection {
	return &webTransportConn{session, perspective}
}

func (c *webTransportConn) AcceptStream(ctx context.Context) (quic.Stream, error) {
	s, err := c.session.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

func (c *webTransportConn) AcceptUniStream(ctx context.Context) (quic.ReceiveStream, error) {
	s, err := c.session.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &ReceiveStream{
		stream: s,
	}, nil
}

func (c *webTransportConn) OpenStream() (quic.Stream, error) {
	s, err := c.session.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

func (c *webTransportConn) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	s, err := c.session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &Stream{
		stream: s,
	}, nil
}

func (c *webTransportConn) OpenUniStream() (quic.SendStream, error) {
	s, err := c.session.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &SendStream{
		stream: s,
	}, nil
}

func (c *webTransportConn) OpenUniStreamSync(ctx context.Context) (quic.SendStream, error) {
	s, err := c.session.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &SendStream{
		stream: s,
	}, nil
}

func (c *webTransportConn) SendDatagram(b []byte) error {
	return c.session.SendDatagram(b)
}

func (c *webTransportConn) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return c.session.ReceiveDatagram(ctx)
}

func (c *webTransportConn) CloseWithError(e uint64, msg string) error {
	return c.session.CloseWithError(webtransport.SessionErrorCode(e), msg)
}

func (c *webTransportConn) Context() context.Context {
	return c.session.Context()
}

func (c *webTransportConn) Protocol() quic.Protocol {
	return quic.ProtocolWebTransport
}

func (c *webTransportConn) ApplicationProtocol() quic.ApplicationProtocol {
	return quic.ApplicationProtocol(c.session.SessionState().ApplicationProtocol)
}

func (c *webTransportConn) Perspective() quic.Perspective {
	return c.perspective
}
