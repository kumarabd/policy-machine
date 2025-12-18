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

// ListSubjects returns paginated list of subjects
// @Summary List subjects
// @Description Returns paginated list of subjects (users)
// @Tags subjects
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size (default: 50, max: 1000)"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} ListSubjectsResponse
// @Router /api/v1/subjects [get]
func (h *BaseServer) ListSubjects(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListSubjects(w, r)
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

	users, nextCursor, hasMore, err := h.engine.GetDB().ListSubjects(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	subjects := make([]api.Subject, len(users))
	for i, u := range users {
		displayName := u.Display
		if displayName == "" {
			displayName = u.ExternalID
		}
		subjects[i] = api.Subject{
			ID:          u.ID,
			ExternalID:  u.ExternalID,
			Email:       u.Email,
			Display:     u.Display,
			DisplayName: displayName,
			Kind:        "user",
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   u.CreatedAt,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(subjects)

	response := api.SearchResponse[api.Subject]{
		Items:      subjects,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateSubject creates a new subject
// @Summary Create subject
// @Description Creates a new subject (user)
// @Tags subjects
// @Accept json
// @Produce json
// @Param request body CreateSubjectRequest true "Subject data"
// @Success 201 {object} SubjectResponse
// @Router /api/v1/subjects [post]
func (h *BaseServer) CreateSubject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateSubject(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req CreateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.ExternalID == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "external_id is required")
		return
	}

	user := &postgres.User{
		ExternalID: req.ExternalID,
		Email:      req.Email,
		Display:    req.Display,
	}

	revision, err := h.engine.GetDB().CreateSubject(r.Context(), tenantID, user)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := SubjectResponse{
		Subject: Subject{
			ID:         user.ID,
			ExternalID: user.ExternalID,
			Email:      user.Email,
			Display:    user.Display,
			CreatedAt:  user.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetSubject returns a subject by ID
// @Summary Get subject
// @Description Returns a subject by ID
// @Tags subjects
// @Produce json
// @Param id path string true "Subject ID"
// @Success 200 {object} Subject
// @Router /api/v1/subjects/{id} [get]
func (h *BaseServer) GetSubject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetSubject(w, r)
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
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	user, err := h.engine.GetDB().GetSubject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	displayName := user.Display
	if displayName == "" {
		displayName = user.ExternalID
	}
	subject := api.Subject{
		ID:          user.ID,
		ExternalID:  user.ExternalID,
		Email:       user.Email,
		Display:     user.Display,
		DisplayName: displayName,
		Kind:        "user",
		Attributes:  make(map[string]string),
		Tags:        []string{},
		CreatedAt:   user.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subject)
}

// UpdateSubject updates a subject
// @Summary Update subject
// @Description Updates a subject
// @Tags subjects
// @Accept json
// @Produce json
// @Param id path string true "Subject ID"
// @Param request body UpdateSubjectRequest true "Update data"
// @Success 200 {object} SubjectResponse
// @Router /api/v1/subjects/{id} [patch]
func (h *BaseServer) UpdateSubject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.UpdateSubject(w, r)
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
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	var req UpdateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Display != "" {
		updates["display"] = req.Display
	}

	if len(updates) == 0 {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No fields to update")
		return
	}

	revision, err := h.engine.GetDB().UpdateSubject(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Fetch updated subject
	user, _ := h.engine.GetDB().GetSubject(r.Context(), tenantID, id)

	displayName := user.Display
	if displayName == "" {
		displayName = user.ExternalID
	}
	response := api.SubjectResponse{
		Subject: api.Subject{
			ID:          user.ID,
			ExternalID:  user.ExternalID,
			Email:       user.Email,
			Display:     user.Display,
			DisplayName: displayName,
			Kind:        "user",
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   user.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteSubject deletes a subject
// @Summary Delete subject
// @Description Deletes a subject
// @Tags subjects
// @Param id path string true "Subject ID"
// @Success 204 "No Content"
// @Router /api/v1/subjects/{id} [delete]
func (h *BaseServer) DeleteSubject(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.DeleteSubject(w, r)
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
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	_, err = h.engine.GetDB().DeleteSubject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSubjectGroups returns paginated list of subject sets
// @Summary List subject sets
// @Description Returns paginated list of subject sets (UAs)
// @Tags subject-sets
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} SearchResponse
// @Router /api/v1/subject-sets [get]
func (h *BaseServer) ListSubjectGroups(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListSubjectGroups(w, r)
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

	uas, nextCursor, hasMore, err := h.engine.GetDB().ListSubjectGroups(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	groups := make([]api.SubjectGroup, len(uas))
	for i, ua := range uas {
		groups[i] = api.SubjectGroup{
			ID:              ua.ID,
			Name:            ua.Name,
			Description:     "",
			ScopeID:         nil,
			Tags:            []string{},
			MemberSubjectIDs: []uuid.UUID{}, // TODO: Populate from relationships
			CreatedAt:       ua.CreatedAt,
			UpdatedAt:       nil,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(groups)

	response := api.SearchResponse[api.SubjectGroup]{
		Items:      groups,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateSubjectGroup creates a new subject set
// @Summary Create subject set
// @Description Creates a new subject set (UA)
// @Tags subject-sets
// @Accept json
// @Produce json
// @Param request body CreateSubjectGroupRequest true "Set data"
// @Success 201 {object} SubjectGroupResponse
// @Router /api/v1/subject-sets [post]
func (h *BaseServer) CreateSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateSubjectGroup(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req CreateSubjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	ua := &postgres.UserAttribute{
		Name: req.Name,
	}

	revision, err := h.engine.GetDB().CreateSubjectGroup(r.Context(), tenantID, ua)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := SubjectGroupResponse{
		Group: SubjectGroup{
			ID:        ua.ID,
			Name:      ua.Name,
			CreatedAt: ua.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetSubjectGroup returns a subject set by ID
// @Summary Get subject set
// @Description Returns a subject set by ID
// @Tags subject-sets
// @Produce json
// @Param id path string true "Set ID"
// @Success 200 {object} SubjectGroup
// @Router /api/v1/subject-sets/{id} [get]
func (h *BaseServer) GetSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetSubjectGroup(w, r)
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

	ua, err := h.engine.GetDB().GetSubjectGroup(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	group := SubjectGroup{
		ID:        ua.ID,
		Name:      ua.Name,
		CreatedAt: ua.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// UpdateSubjectGroup updates a subject set
// @Summary Update subject set
// @Description Updates a subject set
// @Tags subject-sets
// @Accept json
// @Produce json
// @Param id path string true "Set ID"
// @Param request body UpdateSubjectGroupRequest true "Update data"
// @Success 200 {object} SubjectGroupResponse
// @Router /api/v1/subject-sets/{id} [patch]
func (h *BaseServer) UpdateSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.UpdateSubjectGroup(w, r)
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

	var req UpdateSubjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := h.engine.GetDB().UpdateSubjectGroup(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	ua, _ := h.engine.GetDB().GetSubjectGroup(r.Context(), tenantID, id)

	response := SubjectGroupResponse{
		Group: SubjectGroup{
			ID:        ua.ID,
			Name:      ua.Name,
			CreatedAt: ua.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteSubjectGroup deletes a subject set
// @Summary Delete subject set
// @Description Deletes a subject set
// @Tags subject-sets
// @Param id path string true "Set ID"
// @Success 204 "No Content"
// @Router /api/v1/subject-sets/{id} [delete]
func (h *BaseServer) DeleteSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.DeleteSubjectGroup(w, r)
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

	_, err = h.engine.GetDB().DeleteSubjectGroup(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

