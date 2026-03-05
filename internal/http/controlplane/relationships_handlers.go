package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/internal/validate"
	"gorm.io/gorm"
)

// ListRelationships returns paginated list of relationships
func (s *Server) ListRelationships(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	filters := make(map[string]interface{})
	if kind := r.URL.Query().Get("kind"); kind != "" {
		filters["kind"] = kind
	}
	if fromType := r.URL.Query().Get("from_type"); fromType != "" {
		filters["from_type"] = fromType
	}
	if fromIDStr := r.URL.Query().Get("from_id"); fromIDStr != "" {
		if fromID, err := uuid.Parse(fromIDStr); err == nil {
			filters["from_id"] = fromID
		}
	}
	if toType := r.URL.Query().Get("to_type"); toType != "" {
		filters["to_type"] = toType
	}
	if toIDStr := r.URL.Query().Get("to_id"); toIDStr != "" {
		if toID, err := uuid.Parse(toIDStr); err == nil {
			filters["to_id"] = toID
		}
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	edges, nextCursor, hasMore, err := s.engine.GetDB().ListRelationships(r.Context(), tenantID, filters, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	relationships := make([]httputil.Relationship, len(edges))
	for i, edge := range edges {
		// Map edge types to UI types

		relationships[i] = httputil.Relationship{
			ID:   edge.ID,
			Kind: InferRelationshipKind(edge.ChildType, edge.ParentType),
			From: httputil.NodeRef{Type: MapNodeTypeToUI(edge.ChildType), ID: edge.ChildID},
			To:   httputil.NodeRef{Type: MapNodeTypeToUI(edge.ParentType), ID: edge.ParentID},
		}
	}

	response := httputil.ListRelationshipsResponse{
		Relationships: relationships,
		Cursor:        nextCursor,
		HasMore:       hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateRelationship creates a new relationship
func (s *Server) CreateRelationship(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.CreateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Map UI types to NodeTypes
	childType, parentType, err := postgres.MapRelationshipKindToEdgeTypes(req.Kind, req.From.Type, req.To.Type)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	edge := &postgres.AssignmentEdge{
		ChildType:  childType,
		ChildID:    req.From.ID,
		ParentType: parentType,
		ParentID:   req.To.ID,
	}

	revision, err := s.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge)
	if err != nil {
		// Check for validation errors
		if ve, ok := validate.IsValidationError(err); ok {
			if ve.Code == "CYCLE_DETECTED" {
				httputil.RespondError(w, http.StatusConflict, "CYCLE_DETECTED", ve.Message)
				return
			}
			httputil.RespondError(w, http.StatusBadRequest, ve.Code, ve.Message)
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := httputil.RelationshipResponse{
		Relationship: httputil.Relationship{
			ID:   edge.ID,
			Kind: req.Kind,
			From: req.From,
			To:   req.To,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// DeleteRelationship deletes a relationship
func (s *Server) DeleteRelationship(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.DeleteRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	childType, parentType, err := postgres.MapRelationshipKindToEdgeTypes(req.Kind, req.From.Type, req.To.Type)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	edge := &postgres.AssignmentEdge{
		ChildType:  childType,
		ChildID:    req.From.ID,
		ParentType: parentType,
		ParentID:   req.To.ID,
	}

	revision, err := s.engine.GetDB().DeleteRelationship(r.Context(), tenantID, edge)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Relationship not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := httputil.RelationshipResponse{
		Relationship: httputil.Relationship{
			Kind: req.Kind,
			From: req.From,
			To:   req.To,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions (exported for use in other handlers)
// MapNodeTypeToUI maps database NodeType to relationship API type names.
// Note: "subject-set" and "object-set" are API terminology ONLY for the subject-sets/object-sets endpoints.
// In relationships API, we use "subject-attribute" and "object-attribute" to refer to the actual entities.
func MapNodeTypeToUI(nt postgres.NodeType) string {
	switch nt {
	case postgres.NodeSubject:
		return "subject"
	case postgres.NodeUA:
		return "subject-attribute"
	case postgres.NodeObject:
		return "object"
	case postgres.NodeOA:
		return "object-attribute"
	default:
		return string(nt)
	}
}

func InferRelationshipKind(childType, parentType postgres.NodeType) string {
	if childType == postgres.NodeSubject && parentType == postgres.NodeUA {
		return "subject_member_of_attribute"
	}
	if childType == postgres.NodeUA && parentType == postgres.NodeUA {
		return "subject_attribute_parent_of_attribute"
	}
	if childType == postgres.NodeObject && parentType == postgres.NodeOA {
		return "object_member_of_attribute"
	}
	if childType == postgres.NodeOA && parentType == postgres.NodeOA {
		return "object_attribute_parent_of_attribute"
	}
	return "unknown"
}
