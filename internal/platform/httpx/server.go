package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Server wraps http.Server with sane timeouts and graceful shutdown.
type Server struct {
	srv *http.Server
	log *slog.Logger
}

// NewServer builds an HTTP server listening on addr with handler h.
func NewServer(addr string, h http.Handler, log *slog.Logger) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           h,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		log: log,
	}
}

// Start runs the server until ctx is cancelled, then shuts down gracefully
// within the given timeout. It blocks until shutdown completes.
func (s *Server) Start(ctx context.Context, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server listening", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("shutting down http server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	}
}
