package controlplane

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
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

// registerRoutes configures controlplane API endpoints (policy management only)
func (s *Server) registerRoutes(defaultTenantID uuid.UUID) {
	// Add middleware
	s.handler.Use(middleware.Logger)
	s.handler.Use(middleware.Recoverer)
	s.handler.Use(corsMiddleware())
	s.handler.Use(httputil.TenantMiddleware(s.engine, defaultTenantID))

	// Health and metrics endpoints
	s.handler.Get("/healthz", s.HealthHandler)
	s.handler.Get("/readyz", s.HealthHandler)
	s.handler.Get("/metrics", s.MetricsHandler)
	s.handler.Get("/swagger/doc.json", s.SwaggerJSONHandler)

	// Swagger documentation
	s.handler.Route("/swagger", func(r chi.Router) {
		r.Get("/*", httpSwagger.WrapHandler)
	})

	// API v1 routes - Controlplane only (policy management)
	s.handler.Route("/api/v1", func(r chi.Router) {
		// Metadata & Revisions
		r.Get("/meta", s.GetMeta)
		r.Get("/revisions/current", s.GetCurrentRevision)
		r.Get("/changes", s.GetPolicyChanges)
		r.Get("/versions", s.ListVersions)
		r.Get("/versions/{id}/diff", s.GetVersionDiff)
		r.Get("/versions/{id}/snapshot", s.GetVersionSnapshot)

		// Subjects
		r.Route("/subjects", func(r chi.Router) {
			r.Get("/", s.ListSubjects)
			r.Post("/", s.CreateSubject)
			r.Get("/{id}", s.GetSubject)
			r.Patch("/{id}", s.UpdateSubject)
			r.Delete("/{id}", s.DeleteSubject)
		})

		// Subject Sets
		r.Route("/subject-sets", func(r chi.Router) {
			r.Get("/", s.ListSubjectGroups)
			r.Post("/", s.CreateSubjectGroup)
			r.Get("/{id}", s.GetSubjectGroup)
			r.Patch("/{id}", s.UpdateSubjectGroup)
			r.Delete("/{id}", s.DeleteSubjectGroup)
			r.Post("/{id}/members:add", s.AddSubjectSetMembers)
			r.Post("/{id}/members:remove", s.RemoveSubjectSetMember)
		})

		// Objects
		r.Route("/objects", func(r chi.Router) {
			r.Get("/", s.ListObjects)
			r.Post("/", s.CreateObject)
			r.Get("/{id}", s.GetObject)
			r.Patch("/{id}", s.UpdateObject)
			r.Delete("/{id}", s.DeleteObject)
		})

		// Object Sets
		r.Route("/object-sets", func(r chi.Router) {
			r.Get("/", s.ListObjectGroups)
			r.Post("/", s.CreateObjectGroup)
			r.Get("/{id}", s.GetObjectGroup)
			r.Patch("/{id}", s.UpdateObjectGroup)
			r.Delete("/{id}", s.DeleteObjectGroup)
			r.Post("/{id}/members:add", s.AddObjectSetMembers)
			r.Post("/{id}/members:remove", s.RemoveObjectSetMember)
		})

		// Relationships
		r.Route("/relationships", func(r chi.Router) {
			r.Get("/", s.ListRelationships)
			r.Post("/", s.CreateRelationship)
			r.Delete("/", s.DeleteRelationship)
		})

		// Rules (Associations)
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", s.ListRules)
			r.Post("/", s.CreateRule)
			r.Get("/{id}", s.GetRule)
			r.Patch("/{id}", s.UpdateRule)
			r.Delete("/{id}", s.DeleteRule)
		})

		// Denies (Prohibitions)
		r.Route("/denies", func(r chi.Router) {
			r.Get("/", s.ListDenies)
			r.Post("/", s.CreateDeny)
			r.Get("/{id}", s.GetDeny)
			r.Patch("/{id}", s.UpdateDeny)
			r.Delete("/{id}", s.DeleteDeny)
		})

		// Scopes
		r.Get("/scopes", s.ListScopes)

		// Graph
		r.Route("/graph", func(r chi.Router) {
			r.Get("/summary", s.GetGraphSummary)
			r.Get("/neighborhood", s.GetGraphNeighborhood)
			r.Post("/neighborhood", s.GetGraphNeighborhood) // Support POST for UI compatibility
			r.Get("/search", s.GraphSearch)
		})

		// Import/Export
		r.Get("/policy/export", s.ExportPolicy)
		r.Post("/policy/import", s.ImportPolicy)
	})

	// Also register routes under /v1 for backward compatibility
	s.handler.Route("/v1", func(r chi.Router) {
		// Metadata & Revisions
		r.Get("/meta", s.GetMeta)
		r.Get("/revisions/current", s.GetCurrentRevision)
		r.Get("/changes", s.GetPolicyChanges)
		r.Get("/versions", s.ListVersions)
		r.Get("/versions/{id}/diff", s.GetVersionDiff)
		r.Get("/versions/{id}/snapshot", s.GetVersionSnapshot)

		// Subjects
		r.Route("/subjects", func(r chi.Router) {
			r.Get("/", s.ListSubjects)
			r.Post("/", s.CreateSubject)
			r.Get("/{id}", s.GetSubject)
			r.Patch("/{id}", s.UpdateSubject)
			r.Delete("/{id}", s.DeleteSubject)
		})

		// Subject Sets
		r.Route("/subject-sets", func(r chi.Router) {
			r.Get("/", s.ListSubjectGroups)
			r.Post("/", s.CreateSubjectGroup)
			r.Get("/{id}", s.GetSubjectGroup)
			r.Patch("/{id}", s.UpdateSubjectGroup)
			r.Delete("/{id}", s.DeleteSubjectGroup)
			r.Post("/{id}/members:add", s.AddSubjectSetMembers)
			r.Post("/{id}/members:remove", s.RemoveSubjectSetMember)
		})

		// Objects
		r.Route("/objects", func(r chi.Router) {
			r.Get("/", s.ListObjects)
			r.Post("/", s.CreateObject)
			r.Get("/{id}", s.GetObject)
			r.Patch("/{id}", s.UpdateObject)
			r.Delete("/{id}", s.DeleteObject)
		})

		// Object Sets
		r.Route("/object-sets", func(r chi.Router) {
			r.Get("/", s.ListObjectGroups)
			r.Post("/", s.CreateObjectGroup)
			r.Get("/{id}", s.GetObjectGroup)
			r.Patch("/{id}", s.UpdateObjectGroup)
			r.Delete("/{id}", s.DeleteObjectGroup)
			r.Post("/{id}/members:add", s.AddObjectSetMembers)
			r.Post("/{id}/members:remove", s.RemoveObjectSetMember)
		})

		// Relationships
		r.Route("/relationships", func(r chi.Router) {
			r.Get("/", s.ListRelationships)
			r.Post("/", s.CreateRelationship)
			r.Delete("/", s.DeleteRelationship)
		})

		// Rules (Associations)
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", s.ListRules)
			r.Post("/", s.CreateRule)
			r.Get("/{id}", s.GetRule)
			r.Patch("/{id}", s.UpdateRule)
			r.Delete("/{id}", s.DeleteRule)
		})

		// Denies (Prohibitions)
		r.Route("/denies", func(r chi.Router) {
			r.Get("/", s.ListDenies)
			r.Post("/", s.CreateDeny)
			r.Get("/{id}", s.GetDeny)
			r.Patch("/{id}", s.UpdateDeny)
			r.Delete("/{id}", s.DeleteDeny)
		})

		// Scopes
		r.Get("/scopes", s.ListScopes)

		// Graph
		r.Route("/graph", func(r chi.Router) {
			r.Get("/summary", s.GetGraphSummary)
			r.Get("/neighborhood", s.GetGraphNeighborhood)
			r.Post("/neighborhood", s.GetGraphNeighborhood)
			r.Get("/search", s.GraphSearch)
		})

		// Import/Export
		r.Get("/policy/export", s.ExportPolicy)
		r.Post("/policy/import", s.ImportPolicy)
	})
}

// HealthHandler returns the health status of the service
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	httputil.RespondJSON(w, map[string]string{"status": "ok"})
}

// MetricsHandler returns metrics
func (s *Server) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# Metrics endpoint placeholder\n"))
}

// SwaggerJSONHandler serves the swagger.json file
func (s *Server) SwaggerJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// TODO: Implement swagger docs
	w.Write([]byte("{}"))
}

