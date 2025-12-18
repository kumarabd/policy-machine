package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/engine"
)

type contextKey string

const tenantIDKey contextKey = "tenant_id"
const mockModeKey contextKey = "mock_mode"

// TenantMiddleware extracts tenant ID from X-Tenant-ID or X-Tenant-Id header
// Returns 400 if tenant ID is missing or invalid (same behavior for mock and production)
func TenantMiddleware(eng *engine.Engine, defaultTenantID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			// Parse UUID from header
			tenantUUID, err := uuid.Parse(tenantID)
			if err != nil {
				RespondError(w, http.StatusBadRequest, "INVALID_TENANT", "Invalid tenant ID format: must be a valid UUID")
				return
			}

			// Check for X-Mock-Mode header
			mockMode := false
			if mockModeStr := r.Header.Get("X-Mock-Mode"); mockModeStr != "" {
				if val, err := strconv.ParseBool(mockModeStr); err == nil {
					mockMode = val
				} else if strings.ToLower(mockModeStr) == "true" {
					mockMode = true
				}
			}

			// Store tenant ID and mock mode in context
			ctx := context.WithValue(r.Context(), tenantIDKey, tenantUUID)
			ctx = context.WithValue(ctx, mockModeKey, mockMode)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantID extracts tenant ID from request context
func GetTenantID(ctx context.Context) (uuid.UUID, bool) {
	tenantID, ok := ctx.Value(tenantIDKey).(uuid.UUID)
	return tenantID, ok
}

// IsMockMode checks if mock mode is enabled from request context
func IsMockMode(ctx context.Context) bool {
	mockMode, ok := ctx.Value(mockModeKey).(bool)
	return ok && mockMode
}


