package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/kumarabd/policy-machine/pkg/engine"
)

type contextKey string

const tenantIDKey contextKey = "tenant_id"

// TenantMiddleware extracts tenant ID from X-Tenant-ID or X-Tenant-Id header
// Returns 400 if tenant ID is missing
// Excludes swagger endpoints from tenant validation
func TenantMiddleware(eng *engine.Engine, defaultTenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip tenant validation for public endpoints
			path := r.URL.Path
			// Normalize path - remove trailing slash for comparison
			pathNormalized := strings.TrimSuffix(path, "/")
			if strings.HasPrefix(path, "/swagger") ||
				strings.HasPrefix(path, "/swager") || // Handle common typo
				pathNormalized == "/healthz" ||
				pathNormalized == "/readyz" ||
				pathNormalized == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Try both header variants (X-Tenant-ID and X-Tenant-Id)
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = r.Header.Get("X-Tenant-Id")
			}

			// Require tenant ID header - no fallbacks
			if tenantID == "" {
				RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "X-Tenant-ID or X-Tenant-Id header is required")
				return
			}

			// Store tenant ID in context
			ctx := context.WithValue(r.Context(), tenantIDKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantID extracts tenant ID from request context
func GetTenantID(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(tenantIDKey).(string)
	return tenantID, ok
}
