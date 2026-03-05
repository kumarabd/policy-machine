package controlplane

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/api/mapper"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
)

// AddSubjectAttributeMembers adds members to a custom subject attribute.
// "Subject set" is API terminology for SubjectAttribute with attribute_type="custom".
// @Summary Add members to subject set
// @Description Adds one or more subjects to a subject set (custom subject attribute)
// @Tags subject-sets
// @Accept json
// @Produce json
// @Param id path string true "Subject Set ID"
// @Param request body AddMembersRequest true "Member IDs"
// @Success 200 {object} api.SubjectAttribute
// @Router /api/v1/subject-sets/{id}/members:add [post]
func (s *Server) AddSubjectAttributeMembers(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject attribute ID")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.SubjectAttribute(ua))
}

// RemoveSubjectAttributeMember removes a member from a custom subject attribute.
// "Subject set" is API terminology for SubjectAttribute with attribute_type="custom".
// @Summary Remove member from subject set
// @Description Removes a subject from a subject set (custom subject attribute)
// @Tags subject-sets
// @Accept json
// @Produce json
// @Param id path string true "Subject Set ID"
// @Param request body RemoveMemberRequest true "Member ID"
// @Success 200 {object} api.SubjectAttribute
// @Router /api/v1/subject-sets/{id}/members:remove [post]
func (s *Server) RemoveSubjectAttributeMember(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject attribute ID")
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.SubjectAttribute(ua))
}

// AddObjectAttributeMembers adds members to a custom object attribute.
// "Object set" is API terminology for ObjectAttribute with attribute_type="custom".
// @Summary Add members to object set
// @Description Adds one or more objects to an object set (custom object attribute)
// @Tags object-sets
// @Accept json
// @Produce json
// @Param id path string true "Object Set ID"
// @Param request body AddObjectMembersRequest true "Member IDs"
// @Success 200 {object} api.ObjectAttribute
// @Router /api/v1/object-sets/{id}/members:add [post]
func (s *Server) AddObjectAttributeMembers(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object attribute ID")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.ObjectAttribute(oa))
}

// RemoveObjectAttributeMember removes a member from a custom object attribute.
// "Object set" is API terminology for ObjectAttribute with attribute_type="custom".
// @Summary Remove member from object set
// @Description Removes an object from an object set (custom object attribute)
// @Tags object-sets
// @Accept json
// @Produce json
// @Param id path string true "Object Set ID"
// @Param request body RemoveObjectMemberRequest true "Member ID"
// @Success 200 {object} api.ObjectAttribute
// @Router /api/v1/object-sets/{id}/members:remove [post]
func (s *Server) RemoveObjectAttributeMember(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object attribute ID")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.ObjectAttribute(oa))
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
