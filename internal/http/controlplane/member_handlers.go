package controlplane

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/mock"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// AddSubjectSetMembers adds members to a subject set
// @Summary Add members to subject set
// @Description Adds one or more subjects to a subject set
// @Tags subject-sets
// @Accept json
// @Produce json
// @Param id path string true "Subject Set ID"
// @Param request body AddMembersRequest true "Member IDs"
// @Success 200 {object} api.SubjectSet
// @Router /api/v1/subject-sets/{id}/members:add [post]
func (s *Server) AddSubjectSetMembers(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.AddSubjectSetMembers(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject set ID")
		return
	}

	var req AddMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Convert string IDs to UUIDs
	subjectIDs := make([]uuid.UUID, len(req.SubjectIDs))
	for i, idStr := range req.SubjectIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID: "+idStr)
			return
		}
		subjectIDs[i] = id
	}

	// Add assignment edges (SUBJECT -> UA)
	for _, subjectID := range subjectIDs {
		edge := &postgres.AssignmentEdge{
			ChildType:  postgres.NodeSubject,
			ChildID:    subjectID,
			ParentType: postgres.NodeUA,
			ParentID:   setID,
		}
		_, err := s.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	}

	// Refresh engine
	if err := s.engine.Refresh(r.Context()); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "REFRESH_ERROR", err.Error())
		return
	}

	// Fetch updated subject set
	ua, err := s.engine.GetDB().GetSubjectSet(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get member IDs from assignment edges
	memberIDs, err := s.engine.GetDB().GetSubjectSetMembers(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.SubjectSet{
		ID:               ua.ID,
		Name:             ua.Name,
		Description:      "", // Description not stored in SubjectAttribute table
		ScopeID:          nil,
		Tags:             []string{},
		MemberSubjectIDs: memberIDs,
		CreatedAt:        ua.CreatedAt,
		UpdatedAt:        &ua.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RemoveSubjectSetMember removes a member from a subject set
// @Summary Remove member from subject set
// @Description Removes a subject from a subject set
// @Tags subject-sets
// @Accept json
// @Produce json
// @Param id path string true "Subject Set ID"
// @Param request body RemoveMemberRequest true "Member ID"
// @Success 200 {object} api.SubjectSet
// @Router /api/v1/subject-sets/{id}/members:remove [post]
func (s *Server) RemoveSubjectSetMember(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.RemoveSubjectSetMember(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject set ID")
		return
	}

	var req RemoveMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	subjectID, err := uuid.Parse(req.SubjectID)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	// Remove assignment edge (SUBJECT -> UA)
	edge := &postgres.AssignmentEdge{
		ChildType:  postgres.NodeSubject,
		ChildID:    subjectID,
		ParentType: postgres.NodeUA,
		ParentID:   setID,
	}
	_, err = s.engine.GetDB().DeleteRelationship(r.Context(), tenantID, edge)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Refresh engine
	if err := s.engine.Refresh(r.Context()); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "REFRESH_ERROR", err.Error())
		return
	}

	// Fetch updated subject set
	ua, err := s.engine.GetDB().GetSubjectSet(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get member IDs from assignment edges
	memberIDs, err := s.engine.GetDB().GetSubjectSetMembers(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.SubjectSet{
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
	json.NewEncoder(w).Encode(response)
}

// AddObjectSetMembers adds members to an object set
// @Summary Add members to object set
// @Description Adds one or more objects to an object set
// @Tags object-sets
// @Accept json
// @Produce json
// @Param id path string true "Object Set ID"
// @Param request body AddObjectMembersRequest true "Member IDs"
// @Success 200 {object} api.ObjectSet
// @Router /api/v1/object-sets/{id}/members:add [post]
func (s *Server) AddObjectSetMembers(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.AddObjectSetMembers(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object set ID")
		return
	}

	var req AddObjectMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Convert string IDs to UUIDs
	objectIDs := make([]uuid.UUID, len(req.ObjectIDs))
	for i, idStr := range req.ObjectIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID: "+idStr)
			return
		}
		objectIDs[i] = id
	}

	// Add assignment edges (OBJECT -> OA)
	for _, objectID := range objectIDs {
		edge := &postgres.AssignmentEdge{
			ChildType:  postgres.NodeObject,
			ChildID:    objectID,
			ParentType: postgres.NodeOA,
			ParentID:   setID,
		}
		_, err := s.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	}

	// Refresh engine
	if err := s.engine.Refresh(r.Context()); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "REFRESH_ERROR", err.Error())
		return
	}

	// Fetch updated object set
	oa, err := s.engine.GetDB().GetObjectSet(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get member IDs from assignment edges
	memberIDs, err := s.engine.GetDB().GetObjectSetMembers(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.ObjectSet{
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
	json.NewEncoder(w).Encode(response)
}

// RemoveObjectSetMember removes a member from an object set
// @Summary Remove member from object set
// @Description Removes an object from an object set
// @Tags object-sets
// @Accept json
// @Produce json
// @Param id path string true "Object Set ID"
// @Param request body RemoveObjectMemberRequest true "Member ID"
// @Success 200 {object} api.ObjectSet
// @Router /api/v1/object-sets/{id}/members:remove [post]
func (s *Server) RemoveObjectSetMember(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.RemoveObjectSetMember(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object set ID")
		return
	}

	var req RemoveObjectMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	objectID, err := uuid.Parse(req.ObjectID)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	// Remove assignment edge (OBJECT -> OA)
	edge := &postgres.AssignmentEdge{
		ChildType:  postgres.NodeObject,
		ChildID:    objectID,
		ParentType: postgres.NodeOA,
		ParentID:   setID,
	}
	_, err = s.engine.GetDB().DeleteRelationship(r.Context(), tenantID, edge)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Refresh engine
	if err := s.engine.Refresh(r.Context()); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "REFRESH_ERROR", err.Error())
		return
	}

	// Fetch updated object set
	oa, err := s.engine.GetDB().GetObjectSet(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Get member IDs from assignment edges
	memberIDs, err := s.engine.GetDB().GetObjectSetMembers(r.Context(), tenantID, setID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.ObjectSet{
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
	json.NewEncoder(w).Encode(response)
}

// Request types for member management
type AddMembersRequest struct {
	SubjectIDs []string `json:"subjectIds"`
}

type RemoveMemberRequest struct {
	SubjectID string `json:"subjectId"`
}

type AddObjectMembersRequest struct {
	ObjectIDs []string `json:"objectIds"`
}

type RemoveObjectMemberRequest struct {
	ObjectID string `json:"objectId"`
}
