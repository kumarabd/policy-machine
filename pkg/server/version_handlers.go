package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/mock"
)

// GetVersionDiff returns the diff between two versions
// @Summary Get version diff
// @Description Returns the differences between a version and its parent
// @Tags versions
// @Produce json
// @Param id path string true "Version ID"
// @Success 200 {object} VersionDiff
// @Router /api/v1/versions/{id}/diff [get]
func (h *BaseServer) GetVersionDiff(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetVersionDiff(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	versionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid version ID")
		return
	}

	// Get policy changes for this version
	changes, err := h.engine.GetDB().GetPolicyChanges(r.Context(), tenantID, 0, 10000)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Filter and convert changes for this version
	var versionChanges []api.PolicyChangeItem
	for _, ch := range changes {
		if ch.Revision == versionID {
			// Unmarshal JSON payload
			var payload map[string]interface{}
			if len(ch.Payload) > 0 {
				if err := json.Unmarshal(ch.Payload, &payload); err != nil {
					payload = make(map[string]interface{})
				}
			} else {
				payload = make(map[string]interface{})
			}
			
			// Convert postgres.PolicyChange to api.PolicyChangeItem
			versionChanges = append(versionChanges, api.PolicyChangeItem{
				Seq:       ch.Seq,
				Revision:  ch.Revision,
				Kind:      ch.Kind,
				Op:        string(ch.Op),
				Payload:   payload,
				CreatedAt: ch.CreatedAt,
			})
		}
	}

	// Build diff response (simplified - would need more structure in real implementation)
	response := map[string]interface{}{
		"versionId": idStr,
		"changes":   versionChanges,
		"added":     []interface{}{},
		"removed":   []interface{}{},
		"modified":  []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetVersionSnapshot returns the policy state at a specific version
// @Summary Get version snapshot
// @Description Returns the complete policy state at a specific version
// @Tags versions
// @Produce json
// @Param id path string true "Version ID"
// @Success 200 {object} PolicySnapshot
// @Router /api/v1/versions/{id}/snapshot [get]
func (h *BaseServer) GetVersionSnapshot(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetVersionSnapshot(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	_, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid version ID")
		return
	}
	_ = tenantID // TODO: Use tenantID when implementing full versioning

	// For now, return current state (in real implementation, would restore to specific version)
	// This is a placeholder - full implementation would require versioning support
	response := map[string]interface{}{
		"versionId": idStr,
		"subjects":  []interface{}{},
		"objects":   []interface{}{},
		"rules":     []interface{}{},
		"denies":    []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

