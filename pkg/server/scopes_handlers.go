package server

import (
	"encoding/json"
	"net/http"

	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/mock"
)

// ListScopes returns a list of policy scopes
// @Summary List scopes
// @Description Returns a list of policy scopes
// @Tags scopes
// @Produce json
// @Success 200 {object} api.SearchResponse[api.PolicyScope]
// @Router /api/v1/scopes [get]
func (h *BaseServer) ListScopes(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListScopes(w, r)
		return
	}

	_, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	// For now, return empty scopes list
	// In a real implementation, this would query scopes from the database
	scopes := []api.PolicyScope{}

	response := api.SearchResponse[api.PolicyScope]{
		Items: scopes,
		Total: func() *int { v := 0; return &v }(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

