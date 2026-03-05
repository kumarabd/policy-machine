package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/api/mapper"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ListSubjects returns paginated list of subjects
// @Summary List subjects
// @Description Returns paginated list of subjects with optional filtering by attributes
// @Tags generic
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size (default: 50, max: 1000)"
// @Param cursor query string false "Pagination cursor"
// @Param attribute query string false "Filter by attribute ID (can be specified multiple times)"
// @Success 200 {object} ListSubjectsResponse
// @Router /api/v1/subjects [get]
func (s *Server) ListSubjects(w http.ResponseWriter, r *http.Request) {
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

	// Get attribute filters (can be multiple)
	attributeFilters := r.URL.Query()["attribute"]

	subjectsList, nextCursor, hasMore, err := s.engine.GetDB().ListSubjects(r.Context(), tenantID, query, limit, cursor, attributeFilters)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	subjects := make([]api.Subject, len(subjectsList))
	for i := range subjectsList {
		subjects[i] = mapper.Subject(&subjectsList[i])
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
// @Tags generic
// @Accept json
// @Produce json
// @Param request body api.CreateSubjectRequest true "Subject data"
// @Success 201 {object} api.SubjectResponse
// @Router /api/v1/subjects [post]
func (s *Server) CreateSubject(w http.ResponseWriter, r *http.Request) {
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

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	subject := mapper.CreateSubjectRequestToPostgres(req)
	if subject.Tags == nil {
		subject.Tags = datatypes.JSON("{}")
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
		Subject:  mapper.Subject(subject),
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetSubject returns a subject by ID
// @Summary Get subject
// @Description Returns a subject by ID
// @Tags generic
// @Produce json
// @Param id path string true "Subject ID"
// @Success 200 {object} Subject
// @Router /api/v1/subjects/{id} [get]
func (s *Server) GetSubject(w http.ResponseWriter, r *http.Request) {
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

	// Get all attributes (both native and custom) assigned to this subject
	attributes, err := s.engine.GetDB().GetSubjectAttributes(r.Context(), tenantID, id)
	if err != nil {
		// Log error but continue - attributes are optional
		attributes = []postgres.SubjectAttribute{}
	}

	subject := mapper.Subject(subjectDB)
	// Merge assigned attribute names into metadata
	for _, attr := range attributes {
		if subject.Metadata == nil {
			subject.Metadata = make(map[string]string)
		}
		subject.Metadata["attr:"+attr.Name] = string(attr.AttributeType)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subject)
}

// UpdateSubject updates a subject
// @Summary Update subject
// @Description Updates a subject
// @Tags generic
// @Accept json
// @Produce json
// @Param id path string true "Subject ID"
// @Param request body UpdateSubjectRequest true "Update data"
// @Success 200 {object} api.SubjectResponse
// @Router /api/v1/subjects/{id} [patch]
func (s *Server) UpdateSubject(w http.ResponseWriter, r *http.Request) {
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
	if req.Name != "" {
		updates["external_id"] = req.Name
		updates["display"] = req.Name
		updates["display_name"] = req.Name
	}
	if req.Kind != "" {
		updates["kind"] = req.Kind
	}
	if len(req.Metadata) > 0 {
		tagsJSON, _ := json.Marshal(req.Metadata)
		updates["tags"] = datatypes.JSON(tagsJSON)
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

	response := api.SubjectResponse{
		Subject:  mapper.Subject(subjectDB),
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteSubject deletes a subject
// @Summary Delete subject
// @Description Deletes a subject
// @Tags generic
// @Param id path string true "Subject ID"
// @Success 204 "No Content"
// @Router /api/v1/subjects/{id} [delete]
func (s *Server) DeleteSubject(w http.ResponseWriter, r *http.Request) {
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

// ListSubjectAttributesCustom returns paginated list of custom subject attributes (with members).
// @Summary List custom subject attributes
// @Description Returns paginated list of custom subject attributes with member IDs
// @Tags generic
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} SearchResponse
// @Router /api/v1/subject-attributes/custom [get]
func (s *Server) ListSubjectAttributesCustom(w http.ResponseWriter, r *http.Request) {
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

	groups := make([]api.SubjectAttribute, len(uas))
	for i := range uas {
		groups[i] = mapper.SubjectAttribute(&uas[i])
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(groups)

	response := api.SearchResponse[api.SubjectAttribute]{
		Items:      groups,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateSubjectAttributeCustom creates a new custom subject attribute.
// @Summary Create custom subject attribute
// @Description Creates a new custom subject attribute
// @Tags generic
// @Accept json
// @Produce json
// @Param request body api.CreateSubjectAttributeRequest true "Attribute data"
// @Success 201 {object} api.SubjectAttributeResponse
// @Router /api/v1/subject-attributes/custom [post]
func (s *Server) CreateSubjectAttributeCustom(w http.ResponseWriter, r *http.Request) {
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

	revision, err := s.engine.GetDB().CreateSubjectSet(r.Context(), tenantID, ua)
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

// GetSubjectAttributeCustom returns a custom subject attribute by ID (with members).
// @Summary Get custom subject attribute
// @Description Returns a custom subject attribute by ID with member IDs
// @Tags generic
// @Produce json
// @Param id path string true "Attribute ID"
// @Success 200 {object} api.SubjectAttribute
// @Router /api/v1/subject-attributes/custom/{id} [get]
func (s *Server) GetSubjectAttributeCustom(w http.ResponseWriter, r *http.Request) {
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

	ua, err := s.engine.GetDB().GetSubjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Validate that this is a custom attribute
	if ua.AttributeType != postgres.AttributeTypeCustom {
		httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "custom subject attribute not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.SubjectAttribute(ua))
}

// UpdateSubjectAttributeCustom updates a custom subject attribute.
// @Summary Update custom subject attribute
// @Description Updates a custom subject attribute
// @Tags generic
// @Accept json
// @Produce json
// @Param id path string true "Set ID"
// @Param request body api.UpdateSubjectAttributeRequest true "Update data"
// @Success 200 {object} api.SubjectAttributeResponse
// @Router /api/v1/subject-attributes/custom/{id} [patch]
func (s *Server) UpdateSubjectAttributeCustom(w http.ResponseWriter, r *http.Request) {
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

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := s.engine.GetDB().UpdateSubjectSet(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "subject attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	ua, _ := s.engine.GetDB().GetSubjectSet(r.Context(), tenantID, id)

	response := api.SubjectAttributeResponse{
		Attribute: mapper.SubjectAttribute(ua),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteSubjectAttributeCustom deletes a subject set
// @Summary Delete custom subject attribute
// @Description Deletes a custom subject attribute
// @Tags generic
// @Param id path string true "Attribute ID"
// @Success 204 "No Content"
// @Router /api/v1/subject-attributes/custom/{id} [delete]
func (s *Server) DeleteSubjectAttributeCustom(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject attribute ID")
		return
	}

	_, err = s.engine.GetDB().DeleteSubjectSet(r.Context(), tenantID, id)
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

// GetSubjectAttributes returns the complete attribute tree for a subject
// @Summary Get subject attribute tree
// @Description Returns all attributes (native and custom) assigned to a subject and their hierarchical relationships, forming a tree where the subject is the root node
// @Tags generic
// @Produce json
// @Param id path string true "Subject ID"
// @Success 200 {object} api.AttributeSubgraphResponse
// @Router /api/v1/subjects/{id}/attributes [get]
func (s *Server) GetSubjectAttributes(w http.ResponseWriter, r *http.Request) {
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

	attributes, edges, err := s.engine.GetDB().GetSubjectAttributeSubgraph(r.Context(), tenantID, id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Convert to API response format
	nodes := make([]api.AttributeNode, len(attributes))
	for i, attr := range attributes {
		nodes[i] = api.AttributeNode{
			ID:            attr.ID,
			Name:          attr.Name,
			AttributeType: string(attr.AttributeType),
		}
	}

	// Convert edges (include subject->UA edges and UA->UA edges)
	apiEdges := make([]api.AttributeEdge, 0, len(edges))
	for _, edge := range edges {
		// Include subject->UA edges and UA->UA edges
		if (edge.ChildType == postgres.NodeSubject && edge.ParentType == postgres.NodeUA) ||
			(edge.ChildType == postgres.NodeUA && edge.ParentType == postgres.NodeUA) {
			apiEdges = append(apiEdges, api.AttributeEdge{
				ChildID:  edge.ChildID,
				ParentID: edge.ParentID,
			})
		}
	}

	response := api.AttributeSubgraphResponse{
		Nodes: nodes,
		Edges: apiEdges,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
