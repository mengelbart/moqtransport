package transport

import (
	"context"
	"log"

	"github.com/quic-go/webtransport-go"
)

type WebTransportConn struct {
	conn *webtransport.Session
}

func (c *WebTransportConn) OpenStream() (Stream, error) {
	return c.conn.OpenStream()
}

func (c *WebTransportConn) OpenStreamSync(ctx context.Context) (Stream, error) {
	return c.conn.OpenStreamSync(ctx)
}

func (c *WebTransportConn) OpenUniStream() (SendStream, error) {
	return c.conn.OpenUniStream()
}

func (c *WebTransportConn) OpenUniStreamSync(ctx context.Context) (SendStream, error) {
	return c.conn.OpenUniStreamSync(ctx)
}

func (c *WebTransportConn) AcceptStream(ctx context.Context) (Stream, error) {
	return c.conn.AcceptStream(ctx)
}

func (c *WebTransportConn) AcceptUniStream(ctx context.Context) (ReadStream, error) {
	return c.conn.AcceptUniStream(ctx)
}

func (c *WebTransportConn) ReceiveMessage(ctx context.Context) ([]byte, error) {
	panic("ReceiveMessage is not implemented for WebTransport")
}

type WebTransportClient struct {
}

func NewWebTransportClient() *WebTransportClient {
	return &WebTransportClient{}
}
func (c *WebTransportClient) Connect(ctx context.Context, addr string) error {
	var d webtransport.Dialer
	rsp, conn, err := d.Dial(context.TODO(), "https://example.com/webtransport", nil)
	if err != nil {
		return err
	}
	// TODO: Handle rsp?
	log.Println(rsp)
	wc := &WebTransportConn{
		conn: conn,
	}
	_, err = NewClientPeer(ctx, wc)
	if err != nil {
		return err
	}
	return nil
}
