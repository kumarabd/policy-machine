package dataplane

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

// registerRoutes configures dataplane API endpoints (authorization decisions only)
func (s *Server) registerRoutes(defaultTenantID string) {
	// Add common middleware (applies to all routes)
	s.handler.Use(middleware.Logger)
	s.handler.Use(middleware.Recoverer)
	s.handler.Use(corsMiddleware())

	// Public endpoints (no tenant validation required)
	s.handler.Get("/healthz", s.HealthHandler)
	s.handler.Get("/readyz", s.HealthHandler)
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

		// API v1 routes - Dataplane only (authorization decisions)
		r.Route("/api/v1", func(r chi.Router) {
			// Authorization endpoints (fast path)
			r.Post("/authorize", s.Authorize)
			r.Post("/authorize/explain", s.AuthorizeExplain)
			r.Post("/evaluate", s.Evaluate)
		})
	})
}

// HealthHandler returns the health status of the service
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	httputil.RespondJSON(w, map[string]string{"status": "ok"})
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

	// Fallback: provide a complete OpenAPI 3.0 spec with all dataplane endpoints
	minimalSpec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Policy Machine Dataplane API",
			"description": "Authorization API for policy decisions",
			"version":     "1.0.0",
		},
		"paths": map[string]interface{}{
			"/api/v1/authorize": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Authorize a request",
					"description": "Check if a subject has permission to perform an action on an object",
					"tags":        []string{"authorization"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AuthorizeRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Authorization decision",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AuthorizeResponse",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Bad request",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
			},
			"/api/v1/authorize/explain": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Authorize with explanation",
					"description": "Check authorization and get detailed explanation of the decision",
					"tags":        []string{"authorization"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AuthorizeRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Authorization decision with explanation",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ExplainResponse",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Bad request",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
			},
			"/api/v1/evaluate": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Evaluate request",
					"description": "Evaluates if a subject is allowed to perform an action on an object (UI-compatible format)",
					"tags":        []string{"authorization"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/EvaluateRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Evaluation result",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/EvaluateResponse",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Bad request",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"AuthorizeRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"subject_id", "object_id", "operation"},
					"properties": map[string]interface{}{
						"subject_id": map[string]interface{}{
							"type":        "string",
							"format":      "uuid",
							"description": "UUID of the subject/subject",
						},
						"object_id": map[string]interface{}{
							"type":        "string",
							"format":      "uuid",
							"description": "UUID of the object",
						},
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Operation to check (e.g., 'read', 'write', 'delete')",
						},
					},
				},
				"AuthorizeResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"allowed": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the request is allowed",
						},
						"revision": map[string]interface{}{
							"type":        "integer",
							"format":      "int64",
							"description": "Current policy revision",
						},
					},
				},
				"ExplainResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"allowed": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the request is allowed",
						},
						"revision": map[string]interface{}{
							"type":        "integer",
							"format":      "int64",
							"description": "Current policy revision",
						},
						"explain": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"subject_closure": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type":   "string",
										"format": "uuid",
									},
									"description": "Subject closure (all subjects in the closure)",
								},
								"object_closure": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type":   "string",
										"format": "uuid",
									},
									"description": "Object closure (all objects in the closure)",
								},
								"allow_hits": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type":   "string",
										"format": "uuid",
									},
									"description": "Rules that allowed the request",
								},
								"deny_hits": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type":   "string",
										"format": "uuid",
									},
									"description": "Rules that denied the request",
								},
								"effective_allow_set_size": map[string]interface{}{
									"type":        "integer",
									"description": "Size of effective allow set",
								},
								"effective_deny_set_size": map[string]interface{}{
									"type":        "integer",
									"description": "Size of effective deny set",
								},
							},
						},
					},
				},
				"EvaluateRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"subject", "object", "action"},
					"properties": map[string]interface{}{
						"subject": map[string]interface{}{
							"type":     "object",
							"required": []string{"type", "id"},
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"type":        "string",
									"enum":        []string{"SUBJECT", "SUBJECT_SET"},
									"description": "Type of subject",
								},
								"id": map[string]interface{}{
									"type":        "string",
									"description": "Subject ID (UUID)",
								},
							},
						},
						"object": map[string]interface{}{
							"type":     "object",
							"required": []string{"type", "id"},
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"type":        "string",
									"enum":        []string{"OBJECT", "OBJECT_SET"},
									"description": "Type of object",
								},
								"id": map[string]interface{}{
									"type":        "string",
									"description": "Object ID (UUID)",
								},
							},
						},
						"action": map[string]interface{}{
							"type":        "string",
							"description": "Action to evaluate (e.g., 'read', 'write', 'delete')",
						},
						"context": map[string]interface{}{
							"type":                 "object",
							"description":          "Additional context for evaluation",
							"additionalProperties": true,
						},
						"explain": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether to include detailed explanation",
						},
						"atVersionId": map[string]interface{}{
							"type":        "string",
							"description": "Evaluate at a specific version ID",
						},
					},
				},
				"EvaluateResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"decision": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"ALLOW", "DENY"},
							"description": "Authorization decision",
						},
						"versionId": map[string]interface{}{
							"type":        "string",
							"description": "Policy version ID used for evaluation",
						},
						"evaluatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Timestamp of evaluation",
						},
						"trace": map[string]interface{}{
							"type":        "object",
							"description": "Detailed trace (only present if explain=true)",
							"properties": map[string]interface{}{
								"summary": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"matchedRulesCount": map[string]interface{}{
											"type":        "integer",
											"description": "Number of matched rules",
										},
										"denyRulesCount": map[string]interface{}{
											"type":        "integer",
											"description": "Number of deny rules",
										},
										"effectiveDecision": map[string]interface{}{
											"type":        "string",
											"enum":        []string{"ALLOW", "DENY"},
											"description": "Effective decision",
										},
										"versionId": map[string]interface{}{
											"type":        "string",
											"description": "Version ID",
										},
									},
								},
								"matchedRules": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
									},
								},
								"denies": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
									},
								},
							},
						},
					},
				},
			},
			"securitySchemes": map[string]interface{}{
				"TenantHeader": map[string]interface{}{
					"type":        "apiKey",
					"in":          "header",
					"name":        "X-Tenant-ID",
					"description": "Tenant ID for multi-tenancy",
				},
			},
		},
		"security": []map[string]interface{}{
			{
				"TenantHeader": []string{},
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         "http://localhost:8500",
				"description": "Dataplane API server",
			},
		},
	}

	json.NewEncoder(w).Encode(minimalSpec)
}
