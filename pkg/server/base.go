package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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


// RegisterRoutes configures all API endpoints
func (h *BaseServer) RegisterRoutes(defaultTenantID uuid.UUID) {
	// Add middleware
	h.handler.Use(middleware.Logger)
	h.handler.Use(middleware.Recoverer)
	h.handler.Use(corsMiddleware()) // Add CORS support
	h.handler.Use(TenantMiddleware(h.engine, defaultTenantID))

	// Health and metrics endpoints
	h.handler.Get("/healthz", h.HealthHandler)
	h.handler.Get("/readyz", h.HealthHandler)
	h.handler.Get("/metrics", h.MetricsHandler)
	h.handler.Get("/swagger/doc.json", h.SwaggerJSONHandler)

	// Swagger documentation
	h.handler.Route("/swagger", func(r chi.Router) {
		r.Get("/*", httpSwagger.WrapHandler)
	})

	// API v1 routes
	h.handler.Route("/api/v1", func(r chi.Router) {
		// Metadata & Revisions
		r.Get("/meta", h.GetMeta)
		r.Get("/revisions/current", h.GetCurrentRevision)
		r.Get("/changes", h.GetPolicyChanges)
		r.Get("/versions", h.ListVersions)
		r.Get("/versions/{id}/diff", h.GetVersionDiff)
		r.Get("/versions/{id}/snapshot", h.GetVersionSnapshot)

		// Subjects
		r.Route("/subjects", func(r chi.Router) {
			r.Get("/", h.ListSubjects)
			r.Post("/", h.CreateSubject)
			r.Get("/{id}", h.GetSubject)
			r.Patch("/{id}", h.UpdateSubject)
			r.Delete("/{id}", h.DeleteSubject)
		})

		// Subject Sets
		r.Route("/subject-sets", func(r chi.Router) {
			r.Get("/", h.ListSubjectGroups)
			r.Post("/", h.CreateSubjectGroup)
			r.Get("/{id}", h.GetSubjectGroup)
			r.Patch("/{id}", h.UpdateSubjectGroup)
			r.Delete("/{id}", h.DeleteSubjectGroup)
			r.Post("/{id}/members:add", h.AddSubjectSetMembers)
			r.Post("/{id}/members:remove", h.RemoveSubjectSetMember)
		})

		// Objects
		r.Route("/objects", func(r chi.Router) {
			r.Get("/", h.ListObjects)
			r.Post("/", h.CreateObject)
			r.Get("/{id}", h.GetObject)
			r.Patch("/{id}", h.UpdateObject)
			r.Delete("/{id}", h.DeleteObject)
		})

		// Object Sets
		r.Route("/object-sets", func(r chi.Router) {
			r.Get("/", h.ListObjectGroups)
			r.Post("/", h.CreateObjectGroup)
			r.Get("/{id}", h.GetObjectGroup)
			r.Patch("/{id}", h.UpdateObjectGroup)
			r.Delete("/{id}", h.DeleteObjectGroup)
			r.Post("/{id}/members:add", h.AddObjectSetMembers)
			r.Post("/{id}/members:remove", h.RemoveObjectSetMember)
		})

		// Relationships
		r.Route("/relationships", func(r chi.Router) {
			r.Get("/", h.ListRelationships)
			r.Post("/", h.CreateRelationship)
			r.Delete("/", h.DeleteRelationship)
		})

		// Rules (Associations)
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", h.ListRules)
			r.Post("/", h.CreateRule)
			r.Get("/{id}", h.GetRule)
			r.Patch("/{id}", h.UpdateRule)
			r.Delete("/{id}", h.DeleteRule)
		})

		// Denies (Prohibitions)
		r.Route("/denies", func(r chi.Router) {
			r.Get("/", h.ListDenies)
			r.Post("/", h.CreateDeny)
			r.Get("/{id}", h.GetDeny)
			r.Patch("/{id}", h.UpdateDeny)
			r.Delete("/{id}", h.DeleteDeny)
		})

		// Authorization
		r.Post("/authorize", h.Authorize)
		r.Post("/authorize/explain", h.AuthorizeExplain)
		r.Post("/evaluate", h.Evaluate)

		// Scopes
		r.Get("/scopes", h.ListScopes)

		// Graph
		r.Route("/graph", func(r chi.Router) {
			r.Get("/summary", h.GetGraphSummary)
			r.Get("/neighborhood", h.GetGraphNeighborhood)
			r.Get("/search", h.GraphSearch)
		})

		// Import/Export
		r.Get("/policy/export", h.ExportPolicy)
		r.Post("/policy/import", h.ImportPolicy)
	})

	// Also register routes under /v1 for backward compatibility
	h.handler.Route("/v1", func(r chi.Router) {
		// Metadata & Revisions
		r.Get("/meta", h.GetMeta)
		r.Get("/revisions/current", h.GetCurrentRevision)
		r.Get("/changes", h.GetPolicyChanges)
		r.Get("/versions", h.ListVersions) // List versions (different from /changes)
		r.Get("/versions/{id}/diff", h.GetVersionDiff)
		r.Get("/versions/{id}/snapshot", h.GetVersionSnapshot)

		// Subjects
		r.Route("/subjects", func(r chi.Router) {
			r.Get("/", h.ListSubjects)
			r.Post("/", h.CreateSubject)
			r.Get("/{id}", h.GetSubject)
			r.Patch("/{id}", h.UpdateSubject)
			r.Delete("/{id}", h.DeleteSubject)
		})

		// Subject Sets
		r.Route("/subject-sets", func(r chi.Router) {
			r.Get("/", h.ListSubjectGroups)
			r.Post("/", h.CreateSubjectGroup)
			r.Get("/{id}", h.GetSubjectGroup)
			r.Patch("/{id}", h.UpdateSubjectGroup)
			r.Delete("/{id}", h.DeleteSubjectGroup)
			r.Post("/{id}/members:add", h.AddSubjectSetMembers)
			r.Post("/{id}/members:remove", h.RemoveSubjectSetMember)
		})

		// Objects
		r.Route("/objects", func(r chi.Router) {
			r.Get("/", h.ListObjects)
			r.Post("/", h.CreateObject)
			r.Get("/{id}", h.GetObject)
			r.Patch("/{id}", h.UpdateObject)
			r.Delete("/{id}", h.DeleteObject)
		})

		// Object Sets
		r.Route("/object-sets", func(r chi.Router) {
			r.Get("/", h.ListObjectGroups)
			r.Post("/", h.CreateObjectGroup)
			r.Get("/{id}", h.GetObjectGroup)
			r.Patch("/{id}", h.UpdateObjectGroup)
			r.Delete("/{id}", h.DeleteObjectGroup)
			r.Post("/{id}/members:add", h.AddObjectSetMembers)
			r.Post("/{id}/members:remove", h.RemoveObjectSetMember)
		})

		// Relationships
		r.Route("/relationships", func(r chi.Router) {
			r.Get("/", h.ListRelationships)
			r.Post("/", h.CreateRelationship)
			r.Delete("/", h.DeleteRelationship)
		})

		// Rules (Associations)
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", h.ListRules)
			r.Post("/", h.CreateRule)
			r.Get("/{id}", h.GetRule)
			r.Patch("/{id}", h.UpdateRule)
			r.Delete("/{id}", h.DeleteRule)
		})

		// Denies (Prohibitions)
		r.Route("/denies", func(r chi.Router) {
			r.Get("/", h.ListDenies)
			r.Post("/", h.CreateDeny)
			r.Get("/{id}", h.GetDeny)
			r.Patch("/{id}", h.UpdateDeny)
			r.Delete("/{id}", h.DeleteDeny)
		})

		// Authorization
		r.Post("/authorize", h.Authorize)
		r.Post("/authorize/explain", h.AuthorizeExplain)
		r.Post("/evaluate", h.Evaluate)

		// Scopes
		r.Get("/scopes", h.ListScopes)

		// Graph
		r.Route("/graph", func(r chi.Router) {
			r.Get("/summary", h.GetGraphSummary)
			r.Get("/neighborhood", h.GetGraphNeighborhood)
			r.Get("/search", h.GraphSearch)
		})

		// Import/Export
		r.Get("/policy/export", h.ExportPolicy)
		r.Post("/policy/import", h.ImportPolicy)
	})
}
