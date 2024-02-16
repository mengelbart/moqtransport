package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "address to connect to")
	wt := flag.Bool("webtransport", false, "Use webtransport instead of QUIC")
	flag.Parse()

	if err := run(context.Background(), *addr, *wt); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, wt bool) error {
	var session *moqtransport.Session
	var conn moqtransport.Connection
	var err error
	if wt {
		conn, err = dialWebTransport(ctx, addr)
	} else {
		conn, err = dialQUIC(ctx, addr)
	}
	if err != nil {
		return err
	}
	session, err = moqtransport.NewClientSession(conn, moqtransport.IngestionDeliveryRole, true)
	if err != nil {
		return err
	}
	defer session.Close()

	for {
		var a *moqtransport.Announcement
		a, err = session.ReadAnnouncement(context.Background())
		if err != nil {
			panic(err)
		}
		log.Println("got Announcement")
		if a.Namespace() == "clock" {
			a.Accept()
			break
		}
	}

	log.Println("subscribing")
	rs, err := session.Subscribe(context.Background(), 0, 0, "clock", "second", "")
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 64_000)
	for {
		n, err := rs.Read(buf)
		if err != nil {
			panic(err)
		}
		log.Printf("got object: %v\n", string(buf[:n]))
	}
}

func dialQUIC(ctx context.Context, addr string) (moqtransport.Connection, error) {
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{}, &quic.Config{})
	if err != nil {
		return nil, err
	}
	return quicmoq.New(conn), nil
}

func dialWebTransport(ctx context.Context, addr string) (moqtransport.Connection, error) {
	dialer := webtransport.Dialer{
		RoundTripper: &http3.RoundTripper{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			QuicConfig:      &quic.Config{},
			EnableDatagrams: false,
		},
		StreamReorderingTimeout: 0,
	}
	_, session, err := dialer.Dial(ctx, addr, nil)
	if err != nil {
		return nil, err
	}
	return webtransportmoq.New(session), nil
}
