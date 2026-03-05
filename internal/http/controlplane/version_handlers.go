package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// GetVersionDiff returns the diff between two versions
// @Summary Get version diff
// @Description Returns the differences between a version and its parent
// @Tags versions
// @Produce json
// @Param id path string true "Version ID"
// @Success 200 {object} VersionDiff
// @Router /api/v1/versions/{id}/diff [get]
func (s *Server) GetVersionDiff(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	versionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid version ID")
		return
	}

	// Get policy changes for this version
	changes, err := s.engine.GetDB().GetPolicyChanges(r.Context(), tenantID, 0, 10000)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
func (s *Server) GetVersionSnapshot(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetRevision, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid version ID")
		return
	}

	// Get current revision to check if target is valid
	currentRev, err := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if targetRevision > currentRev {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_VERSION", "Version does not exist yet")
		return
	}

	// For version 0, return empty state
	if targetRevision == 0 {
		response := map[string]interface{}{
			"versionId": idStr,
			"subjects":  []interface{}{},
			"objects":   []interface{}{},
			"rules":     []interface{}{},
			"denies":    []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get all changes up to and including the target revision
	// We use a large limit to get all changes (in production, might want pagination)
	changes, err := s.engine.GetDB().GetPolicyChanges(r.Context(), tenantID, 0, 100000)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Filter changes up to target revision
	var relevantChanges []api.PolicyChangeItem
	for _, ch := range changes {
		if ch.Revision <= targetRevision {
			var payload map[string]interface{}
			if len(ch.Payload) > 0 {
				if err := json.Unmarshal(ch.Payload, &payload); err != nil {
					payload = make(map[string]interface{})
				}
			} else {
				payload = make(map[string]interface{})
			}

			relevantChanges = append(relevantChanges, api.PolicyChangeItem{
				Seq:       ch.Seq,
				Revision:  ch.Revision,
				Kind:      ch.Kind,
				Op:        string(ch.Op),
				Payload:   payload,
				CreatedAt: ch.CreatedAt,
			})
		}
	}

	// For now, return the changes that led to this version
	// A full implementation would reconstruct the actual state by applying all changes
	// This is a simplified version that at least provides useful information
	response := map[string]interface{}{
		"versionId": idStr,
		"changes":   relevantChanges,
		"note":      "This is a simplified snapshot. Full state reconstruction would require applying all changes in order.",
		"subjects":  []interface{}{}, // TODO: Reconstruct from changes
		"objects":   []interface{}{}, // TODO: Reconstruct from changes
		"rules":     []interface{}{}, // TODO: Reconstruct from changes
		"denies":    []interface{}{}, // TODO: Reconstruct from changes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
