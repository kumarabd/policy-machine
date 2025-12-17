package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kumarabd/gokit/logger"

	// _ "github.com/kumarabd/policy-machine/docs" // Import docs for swagger
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/engine"
	httpSwagger "github.com/swaggo/http-swagger"
)

type BaseServerConfig struct {
	Port int64 `json:"port" yaml:"port"`
}

type BaseServer struct {
	handler chi.Router
	engine  *engine.Engine
	log     *logger.Handler
	metric  *metrics.Handler
}

// RegisterRoutes configures all API endpoints
//
// API Structure:
// 1. Core APIs: /api/v1/authorize (main endpoint), /api/v1/policies (management)
// 2. Advanced APIs: /api/v1/rbac, /api/v1/abac, /api/v1/rebac (model-specific)
// 3. Internal APIs: /api/v1/ngac (expert-only)
func (h *BaseServer) RegisterRoutes() {
	// Add middleware
	h.handler.Use(middleware.Logger)
	h.handler.Use(middleware.Recoverer)

	// Health and metrics endpoints
	h.handler.Get("/healthz", h.HealthHandler)
	h.handler.Get("/readyz", h.HealthHandler)
	h.handler.Get("/metrics", h.MetricsHandler)
	h.handler.Get("/swagger/doc.json", h.SwaggerJSONHandler)

	// Swagger documentation
	h.handler.Route("/swagger", func(r chi.Router) {
		r.Get("/*", httpSwagger.WrapHandler)
	})
}
