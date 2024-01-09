package moqtransport

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type SessionHandlerFunc func(*Session)

func (h SessionHandlerFunc) Handle(p *Session) {
	h(p)
}

type SessionHandler interface {
	Handle(*Session)
}

type Server struct {
	Handler   SessionHandler
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
			TLSConfig: s.TLSConfig,
		},
		CheckOrigin: func(r *http.Request) bool {
			// TODO: Make configurable
			return true
			// return r.Header.Get("Origin") == "http://localhost:8000"
		},
	}
	l := &wtListener{
		ch: make(chan *webtransport.Session),
	}
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
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
		MaxIdleTimeout:  60 * time.Second,
		EnableDatagrams: true,
	})
	if err != nil {
		return err
	}
	return s.ListenQUICListener(ctx, ql)
}

func (s *Server) ListenQUICListener(ctx context.Context, listener *quic.Listener) error {
	l := &quicListener{
		ql: listener,
	}
	return s.Listen(ctx, l)
}

func (s *Server) Listen(ctx context.Context, l listener) error {
	for {
		conn, err := l.Accept(ctx)
		if err != nil {
			return err
		}
		session, err := newServerSession(ctx, conn)
		if err != nil {
			switch {
			case errors.Is(err, errUnsupportedVersion):
				_ = conn.CloseWithError(SessionTerminatedErrorCode, err.Error())
			case errors.Is(err, errMissingRoleParameter):
				_ = conn.CloseWithError(SessionTerminatedErrorCode, err.Error())
			default:
				_ = conn.CloseWithError(GenericErrorCode, "internal server error")
			}
			continue
		}
		if s.Handler != nil {
			s.Handler.Handle(session)
		}
	}
}
