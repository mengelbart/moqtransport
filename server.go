package moqtransport

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type PeerHandlerFunc func(*Peer)

func (h PeerHandlerFunc) Handle(p *Peer) {
	h(p)
}

type PeerHandler interface {
	Handle(*Peer)
}

type Server struct {
	Handler   PeerHandler
	TLSConfig *tls.Config
}

type listener interface {
	Accept(context.Context) (connection, error)
}

type quicListener struct {
	ql *quic.Listener
}

func (l *quicListener) Accept(ctx context.Context) (connection, error) {
	c, err := l.ql.Accept(ctx)
	if err != nil {
		return nil, err
	}
	qc := &quicConn{
		conn: c,
	}
	return qc, nil
}

type wtListener struct {
	ch chan *webtransport.Session
}

func (l *wtListener) Accept(ctx context.Context) (connection, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case s := <-l.ch:
		wc := &webTransportConn{
			sess: s,
		}
		return wc, nil
	}
}

func (s *Server) ListenWebTransport(ctx context.Context, addr string) error {
	ws := &webtransport.Server{
		H3: http3.Server{
			Addr:      addr,
			Port:      0,
			TLSConfig: s.TLSConfig,
			QuicConfig: &quic.Config{
				GetConfigForClient:             nil,
				Versions:                       nil,
				HandshakeIdleTimeout:           0,
				MaxIdleTimeout:                 1<<63 - 1,
				RequireAddressValidation:       nil,
				TokenStore:                     nil,
				InitialStreamReceiveWindow:     0,
				MaxStreamReceiveWindow:         0,
				InitialConnectionReceiveWindow: 0,
				MaxConnectionReceiveWindow:     0,
				AllowConnectionWindowIncrease:  nil,
				MaxIncomingStreams:             0,
				MaxIncomingUniStreams:          0,
				KeepAlivePeriod:                0,
				DisablePathMTUDiscovery:        false,
				Allow0RTT:                      false,
				EnableDatagrams:                false,
				Tracer:                         nil,
			},
			Handler:            nil,
			EnableDatagrams:    false,
			MaxHeaderBytes:     0,
			AdditionalSettings: map[uint64]uint64{},
			StreamHijacker:     nil,
			UniStreamHijacker:  nil,
		},
		StreamReorderingTimeout: 0,
		CheckOrigin: func(r *http.Request) bool {
			// TODO: Make configurable
			return true
			// return r.Header.Get("Origin") == "http://localhost:8000"
		},
	}
	l := &wtListener{
		ch: make(chan *webtransport.Session),
	}
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("upgrading to WebTransport")
		conn, err := ws.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		select {
		case <-r.Context().Done():
			return
		case l.ch <- conn:
		}
		// Wait for end of request or session termination
		select {
		case <-r.Context().Done():
		case <-conn.Context().Done():
		}
	})
	// TODO: Implement graaceful server shutdown
	errCh := make(chan error)
	go func() {
		if err := ws.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()
	go func() {
		if err := s.Listen(ctx, l); err != nil {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}
	return nil
}

func (s *Server) ListenQUIC(ctx context.Context, addr string) error {
	ql, err := quic.ListenAddr(addr, s.TLSConfig, &quic.Config{
		GetConfigForClient:             nil,
		Versions:                       nil,
		HandshakeIdleTimeout:           0,
		MaxIdleTimeout:                 1<<63 - 1,
		RequireAddressValidation:       nil,
		TokenStore:                     nil,
		InitialStreamReceiveWindow:     0,
		MaxStreamReceiveWindow:         0,
		InitialConnectionReceiveWindow: 0,
		MaxConnectionReceiveWindow:     0,
		AllowConnectionWindowIncrease:  nil,
		MaxIncomingStreams:             0,
		MaxIncomingUniStreams:          0,
		KeepAlivePeriod:                0,
		DisablePathMTUDiscovery:        false,
		Allow0RTT:                      false,
		EnableDatagrams:                true,
		Tracer:                         nil,
	})
	if err != nil {
		return err
	}
	l := &quicListener{
		ql: ql,
	}
	return s.Listen(ctx, l)
}

func (s *Server) Listen(ctx context.Context, l listener) error {
	for {
		conn, err := l.Accept(context.TODO())
		if err != nil {
			return err
		}
		peer, err := newServerPeer(ctx, conn)
		if err != nil {
			log.Printf("failed to create new server peer: %v", err)
			switch {
			case errors.Is(err, errUnsupportedVersion):
				conn.CloseWithError(SessionTerminatedErrorCode, err.Error())
			case errors.Is(err, errMissingRoleParameter):
				conn.CloseWithError(SessionTerminatedErrorCode, err.Error())
			default:
				conn.CloseWithError(GenericErrorCode, "internal server error")
			}
			continue
		}
		if s.Handler != nil {
			s.Handler.Handle(peer)
		}
	}
}
