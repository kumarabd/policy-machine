package dataplane

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/engine"
)

// Config holds dataplane HTTP server configuration
type Config struct {
	Port int64 `json:"port" yaml:"port"`
}

// Server implements the dataplane HTTP server
type Server struct {
	handler chi.Router
	engine  *engine.Engine
	log     *logger.Handler
	metric  *metrics.Handler
	config  *Config
}

// New creates a new dataplane HTTP server instance
func New(l *logger.Handler, m *metrics.Handler, config *Config, eng *engine.Engine) (*Server, error) {
	// Ensure config is initialized
	if config == nil {
		config = &Config{
			Port: 8500, // Default port
		}
	}
	if config.Port == 0 {
		config.Port = 8500 // Default port
	}

	srv := &Server{
		handler: chi.NewRouter(),
		engine:  eng,
		log:     l,
		metric:  m,
		config:  config,
	}

	// Register routes
	defaultTenantID := eng.GetTenantID()
	srv.registerRoutes(defaultTenantID)

	return srv, nil
}

// Start starts the dataplane HTTP server
func (s *Server) Start(ch chan struct{}) error {
	go func() {
		s.log.Info().Msgf("started dataplane http server on port: %d", s.config.Port)
		err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", s.config.Port), s.handler)
		s.log.Error().Err(err).Msg("dataplane server stopped")
		ch <- struct{}{}
	}()
	return nil
}

// Stop stops the dataplane HTTP server
func (s *Server) Stop() error {
	// TODO: Implement graceful shutdown
	return nil
}

// GetEngine returns the engine instance
func (s *Server) GetEngine() *engine.Engine {
	return s.engine
}

// GetHandler returns the chi router (for testing)
func (s *Server) GetHandler() chi.Router {
	return s.handler
}

