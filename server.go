package moqtransport

import (
	"context"
	"crypto/tls"
	"log/slog"
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
	logger    *slog.Logger
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
	return s.ListenWebTransportPath(ctx, addr, "/moq")
}

func (s *Server) ListenWebTransportPath(ctx context.Context, addr, path string) error {
	if s.logger == nil {
		s.logger = defaultLogger.WithGroup("MOQ_SERVER")
	}
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
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("upgrading to WebTransport")
		conn, err := ws.Upgrade(w, r)
		if err != nil {
			s.logger.Error("upgrading failed", "error", err)
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
	if s.logger == nil {
		s.logger = defaultLogger.WithGroup("MOQ_SERVER")
	}
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
	if s.logger == nil {
		s.logger = defaultLogger.WithGroup("MOQ_SERVER")
	}
	l := &quicListener{
		ql: listener,
	}
	return s.Listen(ctx, l)
}

func (s *Server) Listen(ctx context.Context, l listener) error {
	if s.logger == nil {
		s.logger = defaultLogger.WithGroup("MOQ_SERVER")
	}
	for {
		conn, err := l.Accept(ctx)
		if err != nil {
			return err
		}
		go func() {
			if err := s.handleConn(ctx, conn); err != nil {
				s.logger.Error("handle conn failed", "error", err)
			}
		}()
	}
}

func (s *Server) handleConn(ctx context.Context, conn connection) error {
	session, err := newServerSession(ctx, conn, false)
	if err != nil {
		if me, ok := err.(*moqError); ok {
			_ = conn.CloseWithError(uint64(me.code), me.message)
		} else {
			_ = conn.CloseWithError(genericErrorErrorCode, "internal server error")
		}
		return err
	}
	if s.Handler != nil {
		s.Handler.Handle(session)
	}
	return nil
}
