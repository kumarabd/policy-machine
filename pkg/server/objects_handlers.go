package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/mock"
	"github.com/kumarabd/policy-machine/pkg/postgres"
	"gorm.io/gorm"
)

// ListObjects returns paginated list of objects
func (h *BaseServer) ListObjects(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListObjects(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
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

	objects, nextCursor, hasMore, err := h.engine.GetDB().ListObjects(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
func (h *BaseServer) CreateObject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateObject(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req CreateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.ExternalID == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "external_id is required")
		return
	}

	obj := &postgres.Object{
		ExternalID: req.ExternalID,
		Type:       req.Type,
	}

	revision, err := h.engine.GetDB().CreateObject(r.Context(), tenantID, obj)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
func (h *BaseServer) GetObject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetObject(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	obj, err := h.engine.GetDB().GetObject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
func (h *BaseServer) UpdateObject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.UpdateObject(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	var req UpdateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Type != "" {
		updates["type"] = req.Type
	}

	if len(updates) == 0 {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No fields to update")
		return
	}

	revision, err := h.engine.GetDB().UpdateObject(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	obj, _ := h.engine.GetDB().GetObject(r.Context(), tenantID, id)

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
func (h *BaseServer) DeleteObject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.DeleteObject(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	_, err = h.engine.GetDB().DeleteObject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListObjectGroups returns paginated list of object groups
func (h *BaseServer) ListObjectGroups(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListObjectGroups(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
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

	oas, nextCursor, hasMore, err := h.engine.GetDB().ListObjectGroups(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	groups := make([]api.ObjectGroup, len(oas))
	for i, oa := range oas {
		groups[i] = api.ObjectGroup{
			ID:            oa.ID,
			Name:          oa.Name,
			Description:   "",
			ScopeID:       nil,
			Tags:          []string{},
			MemberObjectIDs: []uuid.UUID{}, // TODO: Populate from relationships
			CreatedAt:     oa.CreatedAt,
			UpdatedAt:     nil,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(groups)

	response := api.SearchResponse[api.ObjectGroup]{
		Items:      groups,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateObjectGroup creates a new object group
func (h *BaseServer) CreateObjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateObjectGroup(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req CreateObjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	oa := &postgres.ObjectAttribute{
		Name: req.Name,
	}

	revision, err := h.engine.GetDB().CreateObjectGroup(r.Context(), tenantID, oa)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := ObjectGroupResponse{
		Group: ObjectGroup{
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

// GetObjectGroup returns an object group by ID
func (h *BaseServer) GetObjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetObjectGroup(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	oa, err := h.engine.GetDB().GetObjectGroup(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	group := ObjectGroup{
		ID:        oa.ID,
		Name:      oa.Name,
		CreatedAt: oa.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// UpdateObjectGroup updates an object group
func (h *BaseServer) UpdateObjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.UpdateObjectGroup(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	var req UpdateObjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := h.engine.GetDB().UpdateObjectGroup(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	oa, _ := h.engine.GetDB().GetObjectGroup(r.Context(), tenantID, id)

	response := ObjectGroupResponse{
		Group: ObjectGroup{
			ID:        oa.ID,
			Name:      oa.Name,
			CreatedAt: oa.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteObjectGroup deletes an object group
func (h *BaseServer) DeleteObjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.DeleteObjectGroup(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	_, err = h.engine.GetDB().DeleteObjectGroup(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

