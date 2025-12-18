package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/mock"
)

// GetMeta returns service metadata
// @Summary Get service metadata
// @Description Returns service version, tenant ID, current revision, and applied sequence
// @Tags metadata
// @Produce json
// @Success 200 {object} MetaResponse
// @Router /api/v1/meta [get]
func (h *BaseServer) GetMeta(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetMeta(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	// Get current revision
	db := h.engine.GetDB()
	rev, err := db.GetCurrentRevision(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get applied sequence (last processed change seq)
	// For now, use revision as proxy
	appliedSeq := rev

	response := api.MetaResponse{
		ServiceVersion: "dev", // TODO: inject from config or build-time variable
		TenantID:       tenantID,
		Revision:       rev,
		AppliedSeq:     appliedSeq,
		Now:            time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCurrentRevision returns the current policy revision
// @Summary Get current revision
// @Description Returns the current policy revision for the tenant
// @Tags revisions
// @Produce json
// @Success 200 {object} RevisionResponse
// @Router /api/v1/revisions/current [get]
func (h *BaseServer) GetCurrentRevision(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetCurrentRevision(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	rev, err := h.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.RevisionResponse{
		TenantID: tenantID,
		Revision: rev,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPolicyChanges returns policy changes after a sequence number
// @Summary Get policy changes
// @Description Returns policy changes after the specified sequence number
// @Tags revisions
// @Produce json
// @Param after_seq query int false "Sequence number to start from"
// @Param limit query int false "Maximum number of changes to return (default: 100, max: 1000)"
// @Success 200 {object} ChangesResponse
// @Router /api/v1/changes [get]
func (h *BaseServer) GetPolicyChanges(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetPolicyChanges(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	afterSeq := int64(0)
	if seqStr := r.URL.Query().Get("after_seq"); seqStr != "" {
		if seq, err := strconv.ParseInt(seqStr, 10, 64); err == nil {
			afterSeq = seq
		}
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	changes, err := h.engine.GetDB().GetPolicyChanges(r.Context(), tenantID, afterSeq, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	items := make([]api.PolicyChangeItem, len(changes))
	toSeq := afterSeq
	for i, ch := range changes {
		var payload map[string]interface{}
		json.Unmarshal(ch.Payload, &payload)

		items[i] = api.PolicyChangeItem{
			Seq:       ch.Seq,
			Revision:  ch.Revision,
			Kind:      ch.Kind,
			Op:        string(ch.Op),
			Payload:   payload,
			CreatedAt: ch.CreatedAt,
		}
		if ch.Seq > toSeq {
			toSeq = ch.Seq
		}
	}

	response := api.ChangesResponse{
		TenantID: tenantID,
		FromSeq:  afterSeq,
		ToSeq:    toSeq,
		Changes:  items,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListVersions returns a list of policy versions (derived from revisions)
// @Summary List policy versions
// @Description Returns paginated list of policy versions
// @Tags revisions
// @Produce json
// @Param q query string false "Search query"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size (default: 20, max: 100)"
// @Success 200 {object} ListVersionsResponse
// @Router /api/v1/versions [get]
func (h *BaseServer) ListVersions(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListVersions(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	// Get query params
	query := r.URL.Query().Get("q")
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	// Get policy changes grouped by revision
	changes, err := h.engine.GetDB().GetPolicyChanges(r.Context(), tenantID, 0, 10000) // Get all changes
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Group changes by revision and create versions
	revisionMap := make(map[int64]*api.Version)
	for _, ch := range changes {
		rev := ch.Revision
		if _, exists := revisionMap[rev]; !exists {
			revisionMap[rev] = &api.Version{
				ID:          strconv.FormatInt(rev, 10),
				CreatedAt:   ch.CreatedAt,
				ChangeCount: 0,
			}
			// Set parent version ID (previous revision)
			if rev > 1 {
				parentID := strconv.FormatInt(rev-1, 10)
				revisionMap[rev].ParentVersionID = &parentID
			}
		}
		revisionMap[rev].ChangeCount++
	}

	// Convert map to slice and sort by revision (descending)
	versions := make([]api.Version, 0, len(revisionMap))
	for _, v := range revisionMap {
		versions = append(versions, *v)
	}

	// Simple sorting by revision descending
	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			revI, _ := strconv.ParseInt(versions[i].ID, 10, 64)
			revJ, _ := strconv.ParseInt(versions[j].ID, 10, 64)
			if revI < revJ {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	// Apply search filter if provided
	if query != "" {
		filtered := make([]api.Version, 0)
		queryLower := strings.ToLower(query)
		for _, v := range versions {
			if strings.Contains(strings.ToLower(v.ID), queryLower) ||
				strings.Contains(strings.ToLower(v.Author), queryLower) ||
				strings.Contains(strings.ToLower(v.Message), queryLower) {
				filtered = append(filtered, v)
			}
		}
		versions = filtered
	}

	total := len(versions)

	// Apply pagination
	startIdx := 0
	if cursor != "" {
		if idx, err := strconv.Atoi(cursor); err == nil && idx > 0 && idx < len(versions) {
			startIdx = idx
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(versions) {
		endIdx = len(versions)
	}

	items := versions[startIdx:endIdx]
	var nextCursor *string
	if endIdx < len(versions) {
		cursor := strconv.Itoa(endIdx)
		nextCursor = &cursor
	}

	response := api.SearchResponse[api.Version]{
		Items:      items,
		NextCursor: nextCursor,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
