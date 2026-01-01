package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/mock"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
	"gorm.io/gorm"
)

// ListObjects returns paginated list of objects
func (s *Server) ListObjects(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.ListObjects(w, r)
		return
	}

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

	objects, nextCursor, hasMore, err := s.engine.GetDB().ListObjects(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	objs := make([]api.Object, len(objects))
	for i, o := range objects {
		displayName := o.ExternalID
		if o.Type != "" {
			displayName = o.Type + ": " + o.ExternalID
		}
		objs[i] = api.Object{
			ID:          o.ID,
			ExternalID:  o.ExternalID,
			Type:        o.Type,
			DisplayName: displayName,
			Kind:        o.Type,
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   o.CreatedAt,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(objs)

	response := api.SearchResponse[api.Object]{
		Items:      objs,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateObject creates a new object
func (s *Server) CreateObject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.CreateObject(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.CreateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.ExternalID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "external_id is required")
		return
	}

	obj := &postgres.Object{
		ExternalID: req.ExternalID,
		Type:       req.Type,
	}

	revision, err := s.engine.GetDB().CreateObject(r.Context(), tenantID, obj)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	displayName := obj.ExternalID
	if obj.Type != "" {
		displayName = obj.Type + ": " + obj.ExternalID
	}
	response := api.ObjectResponse{
		Object: api.Object{
			ID:          obj.ID,
			ExternalID:  obj.ExternalID,
			Type:        obj.Type,
			DisplayName: displayName,
			Kind:        obj.Type,
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   obj.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetObject returns an object by ID
func (s *Server) GetObject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.GetObject(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	obj, err := s.engine.GetDB().GetObject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	displayName := obj.ExternalID
	if obj.Type != "" {
		displayName = obj.Type + ": " + obj.ExternalID
	}
	response := api.Object{
		ID:          obj.ID,
		ExternalID:  obj.ExternalID,
		Type:        obj.Type,
		DisplayName: displayName,
		Kind:        obj.Type,
		Attributes:  make(map[string]string),
		Tags:        []string{},
		CreatedAt:   obj.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateObject updates an object
func (s *Server) UpdateObject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.UpdateObject(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	var req httputil.UpdateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Type != "" {
		updates["type"] = req.Type
	}

	if len(updates) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No fields to update")
		return
	}

	revision, err := s.engine.GetDB().UpdateObject(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	obj, _ := s.engine.GetDB().GetObject(r.Context(), tenantID, id)

	displayName := obj.ExternalID
	if obj.Type != "" {
		displayName = obj.Type + ": " + obj.ExternalID
	}
	response := api.ObjectResponse{
		Object: api.Object{
			ID:          obj.ID,
			ExternalID:  obj.ExternalID,
			Type:        obj.Type,
			DisplayName: displayName,
			Kind:        obj.Type,
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   obj.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteObject deletes an object
func (s *Server) DeleteObject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.DeleteObject(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	_, err = s.engine.GetDB().DeleteObject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListObjectSets returns paginated list of object sets
func (s *Server) ListObjectSets(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.ListObjectSets(w, r)
		return
	}

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

	oas, nextCursor, hasMore, err := s.engine.GetDB().ListObjectSets(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	groups := make([]api.ObjectSet, len(oas))
	for i, oa := range oas {
		// Get member IDs for this object set
		memberIDs, err := s.engine.GetDB().GetObjectSetMembers(r.Context(), tenantID, oa.ID)
		if err != nil {
			// Log error but continue with empty member list
			memberIDs = []uuid.UUID{}
		}
		groups[i] = api.ObjectSet{
			ID:              oa.ID,
			Name:            oa.Name,
			Description:     "",
			ScopeID:         nil,
			Tags:            []string{},
			MemberObjectIDs: memberIDs,
			CreatedAt:       oa.CreatedAt,
			UpdatedAt:       nil,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(groups)

	response := api.SearchResponse[api.ObjectSet]{
		Items:      groups,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateObjectSet creates a new object set
func (s *Server) CreateObjectSet(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.CreateObjectSet(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.CreateObjectSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	oa := &postgres.ObjectAttribute{
		Name: req.Name,
	}

	revision, err := s.engine.GetDB().CreateObjectSet(r.Context(), tenantID, oa)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := httputil.ObjectSetResponse{
		Group: api.ObjectSet{
			ID:        oa.ID,
			Name:      oa.Name,
			CreatedAt: oa.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetObjectSet returns an object set by ID
func (s *Server) GetObjectSet(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.GetObjectSet(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	oa, err := s.engine.GetDB().GetObjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object set not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get member IDs from assignment edges
	memberIDs, err := s.engine.GetDB().GetObjectSetMembers(r.Context(), tenantID, id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	group := api.ObjectSet{
		ID:              oa.ID,
		Name:            oa.Name,
		Description:     "",
		ScopeID:         nil,
		Tags:            []string{},
		MemberObjectIDs: memberIDs,
		CreatedAt:       oa.CreatedAt,
		UpdatedAt:       &oa.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// UpdateObjectSet updates an object set
func (s *Server) UpdateObjectSet(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.UpdateObjectSet(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	var req httputil.UpdateObjectSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := s.engine.GetDB().UpdateObjectSet(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object set not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	oa, _ := s.engine.GetDB().GetObjectSet(r.Context(), tenantID, id)

	response := httputil.ObjectSetResponse{
		Group: api.ObjectSet{
			ID:        oa.ID,
			Name:      oa.Name,
			CreatedAt: oa.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteObjectSet deletes an object set
func (s *Server) DeleteObjectSet(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.DeleteObjectSet(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object set ID")
		return
	}

	_, err = s.engine.GetDB().DeleteObjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object set not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
