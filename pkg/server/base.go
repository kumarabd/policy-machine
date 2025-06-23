package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/service"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/kumarabd/policy-machine/docs" // Import docs for swagger
)

type BaseServerConfig struct {
	Port int64 `json:"port" yaml:"port"`
}

type BaseServer struct {
	handler chi.Router
	service *service.Handler
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

	// === MAIN FRONT-FACING APIs ===
	
	// Primary Authorization Endpoint - THE CORE API
	h.handler.Post("/api/v1/authorize", h.AuthorizeHandler)
	
	// Primary Policy Administration APIs
	h.handler.Route("/api/v1/policies", func(r chi.Router) {
		r.Post("/", h.CreatePolicyHandler)
		r.Get("/", h.GetPoliciesHandler)
		r.Put("/{policyId}", h.UpdatePolicyHandler)
		r.Delete("/{policyId}", h.DeletePolicyHandler)
		r.Get("/{policyId}/versions", h.GetPolicyVersionsHandler)
		r.Post("/{policyId}/versions", h.CreatePolicyVersionHandler)
		r.Post("/validate", h.ValidatePolicyHandler)
	})

	// === ADVANCED APIs (for specific access control models) ===

	// RBAC Policy Management APIs (Advanced)
	h.handler.Route("/api/v1/rbac", func(r chi.Router) {
		r.Post("/roles", h.CreateRoleHandler)
		r.Put("/roles/{roleId}", h.UpdateRoleHandler)
		r.Get("/roles", h.GetRolesHandler)
		r.Delete("/roles/{roleId}", h.DeleteRoleHandler)
		r.Post("/permissions", h.CreatePermissionHandler)
		r.Get("/permissions", h.GetPermissionsHandler)
		r.Post("/roles/{roleId}/permissions", h.AssignPermissionHandler)
		r.Delete("/roles/{roleId}/permissions/{permissionId}", h.RemovePermissionHandler)
	})

	// ABAC Policy Management APIs (Advanced)
	h.handler.Route("/api/v1/abac", func(r chi.Router) {
		r.Post("/policies", h.CreateABACPolicyHandler)
		r.Get("/policies", h.GetABACPoliciesHandler)
		r.Put("/policies/{policyId}", h.UpdateABACPolicyHandler)
		r.Delete("/policies/{policyId}", h.DeleteABACPolicyHandler)
		r.Post("/attributes/definitions", h.CreateAttributeDefinitionHandler)
		r.Get("/attributes/definitions", h.GetAttributeDefinitionsHandler)
	})

	// ReBAC Schema Management APIs (Advanced)
	h.handler.Route("/api/v1/rebac", func(r chi.Router) {
		r.Post("/schemas", h.CreateReBACSchemaHandler)
		r.Get("/schemas", h.GetReBACSchemasHandler)
		r.Put("/schemas/{schemaId}", h.UpdateReBACSchemHandler)
		r.Post("/relation-types", h.CreateRelationTypeHandler)
		r.Get("/relation-types", h.GetRelationTypesHandler)
	})

	// === INTERNAL/EXPERT-ONLY APIs ===

	// NGAC Policy Management APIs (Internal/Expert Only)
	h.handler.Route("/api/v1/ngac", func(r chi.Router) {
		r.Post("/policy-classes", h.CreatePolicyClassHandler)
		r.Get("/policy-classes", h.GetPolicyClassesHandler)
		r.Post("/user-attributes", h.CreateUserAttributeHandler)
		r.Post("/object-attributes", h.CreateObjectAttributeHandler)
		r.Post("/associations", h.CreateAssociationHandler)
		r.Get("/graph", h.GetNGACGraphHandler)
	})
}
