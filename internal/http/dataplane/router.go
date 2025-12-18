package dataplane

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
)

// corsMiddleware handles CORS preflight requests
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers - allow all origins for development
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID, X-Tenant-Id, X-Request-Id, X-Mock-Mode")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
			w.Header().Set("Access-Control-Max-Age", "3600")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// registerRoutes configures dataplane API endpoints (authorization decisions only)
func (s *Server) registerRoutes(defaultTenantID uuid.UUID) {
	// Add middleware
	s.handler.Use(middleware.Logger)
	s.handler.Use(middleware.Recoverer)
	s.handler.Use(corsMiddleware())
	s.handler.Use(httputil.TenantMiddleware(s.engine, defaultTenantID))

	// Health endpoints
	s.handler.Get("/healthz", s.HealthHandler)
	s.handler.Get("/readyz", s.HealthHandler)

	// API v1 routes - Dataplane only (authorization decisions)
	s.handler.Route("/api/v1", func(r chi.Router) {
		// Authorization endpoints (fast path)
		r.Post("/authorize", s.Authorize)
		r.Post("/authorize/explain", s.AuthorizeExplain)
		r.Post("/evaluate", s.Evaluate)
	})

	// Also register routes under /v1 for backward compatibility
	s.handler.Route("/v1", func(r chi.Router) {
		// Authorization endpoints (fast path)
		r.Post("/authorize", s.Authorize)
		r.Post("/authorize/explain", s.AuthorizeExplain)
		r.Post("/evaluate", s.Evaluate)
	})
}

// HealthHandler returns the health status of the service
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	httputil.RespondJSON(w, map[string]string{"status": "ok"})
}

