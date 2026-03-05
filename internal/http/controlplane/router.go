package controlplane

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/swaggo/swag"
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID, X-Tenant-Id, X-Request-Id, X-Mock-Mode, ngrok-skip-browser-warning, content-type, authorization")
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
func (s *Server) registerRoutes(defaultTenantID string) {
	// Add common middleware (applies to all routes)
	s.handler.Use(middleware.Logger)
	s.handler.Use(middleware.Recoverer)
	s.handler.Use(corsMiddleware())

	// Public endpoints (no tenant validation required)
	s.handler.Get("/healthz", s.HealthHandler)
	s.handler.Get("/readyz", s.HealthHandler)
	s.handler.Get("/metrics", s.MetricsHandler)
	s.handler.Get("/swagger/doc.json", s.SwaggerJSONHandler)

	// Redirect common typo to correct swagger path
	s.handler.Route("/swager", func(r chi.Router) {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
		})
	})

	// Swagger documentation
	s.handler.Route("/swagger", func(r chi.Router) {
		r.Get("/*", httpSwagger.WrapHandler)
	})

	// API routes with tenant validation
	s.handler.Group(func(r chi.Router) {
		r.Use(httputil.TenantMiddleware(s.engine, defaultTenantID))

		// API v1 routes - Controlplane only (policy management)
		r.Route("/api/v1", func(r chi.Router) {
			// Metadata & Revisions
			r.Get("/meta", s.GetMeta)
			r.Get("/revisions/current", s.GetCurrentRevision)
			r.Get("/changes", s.GetPolicyChanges)
			r.Get("/versions", s.ListVersions)
			r.Get("/versions/{id}/diff", s.GetVersionDiff)
			r.Get("/versions/{id}/snapshot", s.GetVersionSnapshot)

			// RBAC group
			r.Route("/rbac", func(r chi.Router) {
				// Subjects
				r.Post("/subjects", s.CreateRBACSubject)
				r.Post("/subjects:upsert", s.UpsertRBACSubjects)
				r.Get("/subjects/{id}", s.GetRBACSubject)

				// Roles
				r.Get("/roles", s.ListRBACRoles)
				r.Post("/roles", s.CreateRBACRole)
				r.Post("/roles:upsert", s.UpsertRBACRoles)

				// Bindings
				r.Post("/bindings", s.CreateRBACBinding)
				r.Post("/bindings:upsert", s.UpsertRBACBindings)

				// Objects
				r.Get("/objects", s.ListRBACObjects)
				r.Post("/objects:upsert", s.UpsertRBACObjects)
			})

			// Generic resource APIs (subjects, objects, attributes)
			r.Route("/generic", func(r chi.Router) {
				// Subjects
				r.Route("/subjects", func(r chi.Router) {
					r.Get("/", s.ListSubjects)
					r.Post("/", s.CreateSubject)
					r.Get("/{id}", s.GetSubject)
					r.Get("/{id}/attributes", s.GetSubjectAttributes) // Get all attributes for a subject
					r.Patch("/{id}", s.UpdateSubject)
					r.Delete("/{id}", s.DeleteSubject)
				})

				// Subject Attributes
				r.Route("/subject-attributes", func(r chi.Router) {
					r.Get("/", s.ListSubjectAttributes)
					r.Post("/", s.CreateSubjectAttribute)
					r.Get("/{id}", s.GetSubjectAttribute)
					r.Patch("/{id}", s.UpdateSubjectAttribute)
					r.Delete("/{id}", s.DeleteSubjectAttribute)
					// Custom attributes (with member management)
					r.Route("/custom", func(r chi.Router) {
						r.Get("/", s.ListSubjectAttributesCustom)
						r.Post("/", s.CreateSubjectAttributeCustom)
						r.Get("/{id}", s.GetSubjectAttributeCustom)
						r.Patch("/{id}", s.UpdateSubjectAttributeCustom)
						r.Delete("/{id}", s.DeleteSubjectAttributeCustom)
						r.Post("/{id}/members:add", s.AddSubjectAttributeMembers)
						r.Post("/{id}/members:remove", s.RemoveSubjectAttributeMember)
					})
				})

				// Objects
				r.Route("/objects", func(r chi.Router) {
					r.Get("/", s.ListObjects)
					r.Post("/", s.CreateObject)
					r.Get("/{id}", s.GetObject)
					r.Get("/{id}/attributes", s.GetObjectAttributes) // Get all attributes for an object
					r.Patch("/{id}", s.UpdateObject)
					r.Delete("/{id}", s.DeleteObject)
				})

				// Object Attributes
				r.Route("/object-attributes", func(r chi.Router) {
					r.Get("/", s.ListObjectAttributes)
					r.Post("/", s.CreateObjectAttribute)
					r.Get("/{id}", s.GetObjectAttribute)
					r.Patch("/{id}", s.UpdateObjectAttribute)
					r.Delete("/{id}", s.DeleteObjectAttribute)
					// Custom attributes (with member management)
					r.Route("/custom", func(r chi.Router) {
						r.Get("/", s.ListObjectAttributesCustom)
						r.Post("/", s.CreateObjectAttributeCustom)
						r.Get("/{id}", s.GetObjectAttributeCustom)
						r.Patch("/{id}", s.UpdateObjectAttributeCustom)
						r.Delete("/{id}", s.DeleteObjectAttributeCustom)
						r.Post("/{id}/members:add", s.AddObjectAttributeMembers)
						r.Post("/{id}/members:remove", s.RemoveObjectAttributeMember)
					})
				})
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

	// Try to get swagger docs from the swag registry
	if spec := swag.GetSwagger("swagger"); spec != nil {
		doc := spec.ReadDoc()
		w.Write([]byte(doc))
		return
	}

	// Fallback: provide a minimal valid OpenAPI 3.0 spec
	minimalSpec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Policy Machine Controlplane API",
			"description": "Policy management API for subjects, objects, relationships, and rules",
			"version":     "1.0.0",
		},
		"paths": map[string]interface{}{
			"/api/v1/subjects": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List subjects",
					"description": "Get a list of all subjects",
					"tags":        []string{"subjects"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "List of subjects",
						},
					},
				},
				"post": map[string]interface{}{
					"summary":     "Create subject",
					"description": "Create a new subject",
					"tags":        []string{"subjects"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created subject",
						},
					},
				},
			},
		},
		"servers": []map[string]interface{}{
			{
				"url": "http://localhost:8501",
			},
		},
	}

	json.NewEncoder(w).Encode(minimalSpec)
}
