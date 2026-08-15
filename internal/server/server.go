package server

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/scouser-122/meeting-analyzer/internal/config"
)

// Server entity for processing http requests
type Server struct {
	httpServer *http.Server
	config     *config.ServerConfig
	wg         sync.WaitGroup
}

// NewServer creates new server entity
func NewServer(config *config.ServerConfig) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Init(handlers []Handler) error {
	r := http.NewServeMux()
	err := s.addHandlersForRouter(r, &handlers, s.config)
	if err != nil {
		return err
	}
	s.httpServer = &http.Server{
		Addr:    *s.config.RunAddr,
		Handler: r,
	}
	return nil
}

func (s *Server) addHandlersForRouter(r *http.ServeMux, handlers *[]Handler, config *config.ServerConfig) error {
	for _, h := range *handlers {
		r.HandleFunc(h.URLPathPattern, RequestLogger(h.HandlerFn, config))
	}
	return nil
}

// Start starts server
func (s *Server) Start() error {
	slog.Info("starting server", "address", *s.config.RunAddr)
	return s.httpServer.ListenAndServe()
}

// Shutdown shuts down server
func (s *Server) Shutdown() error {
	slog.Info("shutting down server ", "address", *s.config.RunAddr)

	ctx, cancel := context.WithTimeout(context.Background(), *s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}

	slog.Info("server gracefully stopped")
	return nil
}
