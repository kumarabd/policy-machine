package server

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/authz"
	"github.com/kumarabd/policy-machine/pkg/service"
)

type Config struct {
	HTTP HTTPServerConfig `json:"http,omitempty" yaml:"http,omitempty"`
}

type Handler struct {
	HTTPServer *HTTPServer

	config *Config
	log    *logger.Handler
}

func New(l *logger.Handler, config *Config, service *service.Handler) (*Handler, error) {
	// Get OPA URL from environment
	opaURL := os.Getenv("OPA_URL")
	if opaURL == "" {
		opaURL = "http://localhost:8181"
	}

	// Create OPA client
	authzClient := authz.NewClient(opaURL)

	httpObj := &HTTPServer{
		service:     service,
		authzClient: authzClient,
	}

	// Initiate HTTP Server object
	httpObj.handler = gin.New()
	// Global middleware
	httpObj.handler.Use(gin.Recovery())
	gin.SetMode(gin.ReleaseMode)

	// Health and metrics endpoints (no auth required)
	httpObj.handler.GET("/healthz", httpObj.HealthHandler)
	httpObj.handler.GET("/readyz", httpObj.HealthHandler)
	httpObj.handler.GET("/metrics", httpObj.MetricsHandler)

	// Protected routes with authorization middleware
	protected := httpObj.handler.Group("/api/v1")
	protected.Use(authz.Middleware(authzClient))
	{
		protected.GET("/users/:resource_id/data", httpObj.UserDataHandler)
	}

	return &Handler{
		HTTPServer: httpObj,
		config:     config,
		log:        l,
	}, nil
}

func (h *Handler) Run(ch chan struct{}) {
	go func() {
		h.log.Info().Msgf("started http server on port: %s", h.config.HTTP.Port)
		err := h.HTTPServer.handler.Run(fmt.Sprintf("0.0.0.0:%s", h.config.HTTP.Port))
		h.log.Error().Err(err).Msg("unable to start http server")
		ch <- struct{}{}
	}()
}
