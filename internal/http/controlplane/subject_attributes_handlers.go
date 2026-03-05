package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kumarabd/policy-machine/internal/api/mapper"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// ListSubjectAttributes returns all subject attributes (both native and custom)
func (s *Server) ListSubjectAttributes(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	query := r.URL.Query().Get("query")
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	uas, nextCursor, hasMore, err := s.engine.GetDB().ListSubjectAttributes(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Convert postgres -> API via mapper
	attributes := make([]api.SubjectAttribute, len(uas))
	for i := range uas {
		attributes[i] = mapper.SubjectAttribute(&uas[i])
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(attributes)

	response := api.SearchResponse[api.SubjectAttribute]{
		Items:      attributes,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateSubjectAttribute creates a subject attribute (idempotent - creates if doesn't exist)
// If parent_name or parent_id is provided, creates hierarchy edge (UA -> UA)
func (s *Server) CreateSubjectAttribute(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.CreateSubjectAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	ua := mapper.CreateSubjectAttributeRequestToPostgres(req)

	revision, err := s.engine.GetDB().CreateSubjectAttributeWithParent(r.Context(), tenantID, ua, req.ParentName, req.ParentID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.SubjectAttributeResponse{
		Attribute: mapper.SubjectAttribute(ua),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetSubjectAttribute returns a subject attribute by ID
func (s *Server) GetSubjectAttribute(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid attribute ID")
		return
	}

	ua, err := s.engine.GetDB().GetSubjectAttribute(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.SubjectAttribute(ua))
}

// UpdateSubjectAttribute updates a subject attribute
func (s *Server) UpdateSubjectAttribute(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid attribute ID")
		return
	}

	var req api.UpdateSubjectAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}

	if len(updates) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No fields to update")
		return
	}

	revision, err := s.engine.GetDB().UpdateSubjectAttribute(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	ua, _ := s.engine.GetDB().GetSubjectAttribute(r.Context(), tenantID, id)

	response := api.SubjectAttributeResponse{
		Attribute: mapper.SubjectAttribute(ua),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteSubjectAttribute deletes a subject attribute
func (s *Server) DeleteSubjectAttribute(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid attribute ID")
		return
	}

	_, err = s.engine.GetDB().DeleteSubjectAttribute(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
