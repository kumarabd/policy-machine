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
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// ListObjectAttributes returns all object attributes (both native and custom)
func (s *Server) ListObjectAttributes(w http.ResponseWriter, r *http.Request) {
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

	oas, nextCursor, hasMore, err := s.engine.GetDB().ListObjectAttributes(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Convert postgres -> API via mapper
	attributes := make([]api.ObjectAttribute, len(oas))
	for i := range oas {
		attributes[i] = mapper.ObjectAttribute(&oas[i])
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(attributes)

	response := api.SearchResponse[api.ObjectAttribute]{
		Items:      attributes,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateObjectAttribute creates an object attribute (idempotent - creates if doesn't exist)
// If parent_name or parent_id is provided, creates hierarchy edge (OA -> OA)
func (s *Server) CreateObjectAttribute(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.CreateObjectAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	oa := mapper.CreateObjectAttributeRequestToPostgres(req)
	oa.AttributeType = postgres.AttributeTypeCustom

	revision, err := s.engine.GetDB().CreateObjectAttributeWithParent(r.Context(), tenantID, oa, req.ParentName, req.ParentID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.ObjectAttributeResponse{
		Attribute: mapper.ObjectAttribute(oa),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetObjectAttribute returns an object attribute by ID
func (s *Server) GetObjectAttribute(w http.ResponseWriter, r *http.Request) {
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

	oa, err := s.engine.GetDB().GetObjectAttribute(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.ObjectAttribute(oa))
}

// UpdateObjectAttribute updates an object attribute
func (s *Server) UpdateObjectAttribute(w http.ResponseWriter, r *http.Request) {
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

	var req api.UpdateObjectAttributeRequest
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

	revision, err := s.engine.GetDB().UpdateObjectAttribute(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	oa, _ := s.engine.GetDB().GetObjectAttribute(r.Context(), tenantID, id)

	response := api.ObjectAttributeResponse{
		Attribute: mapper.ObjectAttribute(oa),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteObjectAttribute deletes an object attribute
func (s *Server) DeleteObjectAttribute(w http.ResponseWriter, r *http.Request) {
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

	_, err = s.engine.GetDB().DeleteObjectAttribute(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
