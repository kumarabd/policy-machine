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

// ListSubjects returns paginated list of subjects
// @Summary List subjects
// @Description Returns paginated list of subjects
// @Tags subjects
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size (default: 50, max: 1000)"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} ListSubjectsResponse
// @Router /api/v1/subjects [get]
func (s *Server) ListSubjects(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.ListSubjects(w, r)
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

	subjectsList, nextCursor, hasMore, err := s.engine.GetDB().ListSubjects(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	subjects := make([]api.Subject, len(subjectsList))
	for i, s := range subjectsList {
		displayName := s.Display
		if displayName == "" {
			displayName = s.ExternalID
		}
		subjects[i] = api.Subject{
			ID:          s.ID,
			ExternalID:  s.ExternalID,
			Email:       s.Email,
			Display:     s.Display,
			DisplayName: displayName,
			Kind:        "subject",
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   s.CreatedAt,
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
// @Description Creates a new subject
// @Tags subjects
// @Accept json
// @Produce json
// @Param request body api.CreateSubjectRequest true "Subject data"
// @Success 201 {object} api.SubjectResponse
// @Router /api/v1/subjects [post]
func (s *Server) CreateSubject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.CreateSubject(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.CreateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.ExternalID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "external_id is required")
		return
	}

	subject := &postgres.Subject{
		ExternalID: req.ExternalID,
		Email:      req.Email,
		Display:    req.Display,
	}

	revision, err := s.engine.GetDB().CreateSubject(r.Context(), tenantID, subject)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.SubjectResponse{
		Subject: api.Subject{
			ID:         subject.ID,
			ExternalID: subject.ExternalID,
			Email:      subject.Email,
			Display:    subject.Display,
			CreatedAt:  subject.CreatedAt,
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
func (s *Server) GetSubject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.GetSubject(w, r)
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
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	subjectDB, err := s.engine.GetDB().GetSubject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	displayName := subjectDB.Display
	if displayName == "" {
		displayName = subjectDB.ExternalID
	}
	subject := api.Subject{
		ID:          subjectDB.ID,
		ExternalID:  subjectDB.ExternalID,
		Email:       subjectDB.Email,
		Display:     subjectDB.Display,
		DisplayName: displayName,
		Kind:        "subject",
		Attributes:  make(map[string]string),
		Tags:        []string{},
		CreatedAt:   subjectDB.CreatedAt,
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
// @Success 200 {object} api.SubjectResponse
// @Router /api/v1/subjects/{id} [patch]
func (s *Server) UpdateSubject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.UpdateSubject(w, r)
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
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	var req httputil.UpdateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
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
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No fields to update")
		return
	}

	revision, err := s.engine.GetDB().UpdateSubject(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Fetch updated subject
	subjectDB, _ := s.engine.GetDB().GetSubject(r.Context(), tenantID, id)

	displayName := subjectDB.Display
	if displayName == "" {
		displayName = subjectDB.ExternalID
	}
	response := api.SubjectResponse{
		Subject: api.Subject{
			ID:          subjectDB.ID,
			ExternalID:  subjectDB.ExternalID,
			Email:       subjectDB.Email,
			Display:     subjectDB.Display,
			DisplayName: displayName,
			Kind:        "subject",
			Attributes:  make(map[string]string),
			Tags:        []string{},
			CreatedAt:   subjectDB.CreatedAt,
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
func (s *Server) DeleteSubject(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.DeleteSubject(w, r)
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
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	_, err = s.engine.GetDB().DeleteSubject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
func (s *Server) ListSubjectGroups(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.ListSubjectGroups(w, r)
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

	uas, nextCursor, hasMore, err := s.engine.GetDB().ListSubjectSets(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	groups := make([]api.SubjectSet, len(uas))
	for i, ua := range uas {
		// Get member IDs for this subject set
		memberIDs, err := s.engine.GetDB().GetSubjectSetMembers(r.Context(), tenantID, ua.ID)
		if err != nil {
			// Log error but continue with empty member list
			memberIDs = []uuid.UUID{}
		}
		groups[i] = api.SubjectSet{
			ID:               ua.ID,
			Name:             ua.Name,
			Description:      "",
			ScopeID:          nil,
			Tags:             []string{},
			MemberSubjectIDs: memberIDs,
			CreatedAt:        ua.CreatedAt,
			UpdatedAt:        nil,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(groups)

	response := api.SearchResponse[api.SubjectSet]{
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
// @Success 201 {object} api.SubjectGroupResponse
// @Router /api/v1/subject-sets [post]
func (s *Server) CreateSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.CreateSubjectGroup(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.CreateSubjectSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	ua := &postgres.UserAttribute{
		Name: req.Name,
	}

	revision, err := s.engine.GetDB().CreateSubjectSet(r.Context(), tenantID, ua)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.SubjectGroupResponse{
		Group: api.SubjectSet{
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
// @Success 200 {object} api.SubjectSet
// @Router /api/v1/subject-sets/{id} [get]
func (s *Server) GetSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.GetSubjectGroup(w, r)
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

	ua, err := s.engine.GetDB().GetSubjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject set not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get member IDs from assignment edges
	memberIDs, err := s.engine.GetDB().GetSubjectSetMembers(r.Context(), tenantID, id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	group := api.SubjectSet{
		ID:               ua.ID,
		Name:             ua.Name,
		Description:      "",
		ScopeID:          nil,
		Tags:             []string{},
		MemberSubjectIDs: memberIDs,
		CreatedAt:        ua.CreatedAt,
		UpdatedAt:        &ua.UpdatedAt,
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
// @Success 200 {object} api.SubjectGroupResponse
// @Router /api/v1/subject-sets/{id} [patch]
func (s *Server) UpdateSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.UpdateSubjectGroup(w, r)
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

	var req httputil.UpdateSubjectSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := s.engine.GetDB().UpdateSubjectSet(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject set not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	ua, _ := s.engine.GetDB().GetSubjectGroup(r.Context(), tenantID, id)

	response := api.SubjectGroupResponse{
		Group: api.SubjectSet{
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
func (s *Server) DeleteSubjectGroup(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.DeleteSubjectGroup(w, r)
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

	_, err = s.engine.GetDB().DeleteSubjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject set not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
