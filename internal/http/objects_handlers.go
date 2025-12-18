package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/internal/mock"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"gorm.io/gorm"
)

// ListObjects returns paginated list of objects
func (s *HTTP) ListObjects(w http.ResponseWriter, r *http.Request) {
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

	objects, nextCursor, hasMore, err := s.engine.GetDB().ListObjects(r.Context(), tenantID, query, limit, cursor)
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
func (s *HTTP) CreateObject(w http.ResponseWriter, r *http.Request) {
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

	revision, err := s.engine.GetDB().CreateObject(r.Context(), tenantID, obj)
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
func (s *HTTP) GetObject(w http.ResponseWriter, r *http.Request) {
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

	obj, err := s.engine.GetDB().GetObject(r.Context(), tenantID, id)
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
func (s *HTTP) UpdateObject(w http.ResponseWriter, r *http.Request) {
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

	revision, err := s.engine.GetDB().UpdateObject(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
func (s *HTTP) DeleteObject(w http.ResponseWriter, r *http.Request) {
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

	_, err = s.engine.GetDB().DeleteObject(r.Context(), tenantID, id)
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

// ListObjectGroups returns paginated list of object sets
func (s *HTTP) ListObjectGroups(w http.ResponseWriter, r *http.Request) {
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

	oas, nextCursor, hasMore, err := s.engine.GetDB().ListObjectGroups(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	groups := make([]api.ObjectSet, len(oas))
	for i, oa := range oas {
		groups[i] = api.ObjectSet{
			ID:              oa.ID,
			Name:            oa.Name,
			Description:     "",
			ScopeID:         nil,
			Tags:            []string{},
			MemberObjectIDs: []uuid.UUID{}, // TODO: Populate from relationships
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

// CreateObjectGroup creates a new object set
func (s *HTTP) CreateObjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateObjectGroup(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req CreateObjectSetRequest
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

	revision, err := s.engine.GetDB().CreateObjectGroup(r.Context(), tenantID, oa)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := ObjectSetResponse{
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

// GetObjectGroup returns an object set by ID
func (s *HTTP) GetObjectGroup(w http.ResponseWriter, r *http.Request) {
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

	oa, err := s.engine.GetDB().GetObjectGroup(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "object set not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	group := api.ObjectSet{
		ID:        oa.ID,
		Name:      oa.Name,
		CreatedAt: oa.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// UpdateObjectGroup updates an object set
func (s *HTTP) UpdateObjectGroup(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateObjectSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := s.engine.GetDB().UpdateObjectGroup(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "object set not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	oa, _ := s.engine.GetDB().GetObjectGroup(r.Context(), tenantID, id)

	response := ObjectSetResponse{
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

// DeleteObjectGroup deletes an object set
func (s *HTTP) DeleteObjectGroup(w http.ResponseWriter, r *http.Request) {
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

	_, err = s.engine.GetDB().DeleteObjectGroup(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "object set not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
