package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/mock"
	"github.com/kumarabd/policy-machine/pkg/postgres"
	"github.com/kumarabd/policy-machine/pkg/postgres/validate"
	"gorm.io/gorm"
)

// ListRelationships returns paginated list of relationships
func (h *BaseServer) ListRelationships(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListRelationships(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
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

	edges, nextCursor, hasMore, err := h.engine.GetDB().ListRelationships(r.Context(), tenantID, filters, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	relationships := make([]Relationship, len(edges))
	for i, edge := range edges {
		// Map edge types to UI types

		relationships[i] = Relationship{
			ID:   edge.ID,
			Kind: InferRelationshipKind(edge.ChildType, edge.ParentType),
			From: NodeRef{Type: MapNodeTypeToUI(edge.ChildType), ID: edge.ChildID},
			To:   NodeRef{Type: MapNodeTypeToUI(edge.ParentType), ID: edge.ParentID},
		}
	}

	response := ListRelationshipsResponse{
		Relationships: relationships,
		Cursor:        nextCursor,
		HasMore:       hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateRelationship creates a new relationship
func (h *BaseServer) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateRelationship(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req CreateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Map UI types to NodeTypes
	childType, parentType, err := postgres.MapRelationshipKindToEdgeTypes(req.Kind, req.From.Type, req.To.Type)
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	edge := &postgres.AssignmentEdge{
		ChildType:  childType,
		ChildID:    req.From.ID,
		ParentType: parentType,
		ParentID:   req.To.ID,
	}

	revision, err := h.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge)
	if err != nil {
		// Check for validation errors
		if ve, ok := err.(*validate.ValidationError); ok {
			if ve.Code == "CYCLE_DETECTED" {
				respondError(w, http.StatusConflict, "CYCLE_DETECTED", ve.Message)
				return
			}
			respondError(w, http.StatusBadRequest, ve.Code, ve.Message)
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := RelationshipResponse{
		Relationship: Relationship{
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
func (h *BaseServer) DeleteRelationship(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.DeleteRelationship(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req DeleteRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	childType, parentType, err := postgres.MapRelationshipKindToEdgeTypes(req.Kind, req.From.Type, req.To.Type)
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	edge := &postgres.AssignmentEdge{
		ChildType:  childType,
		ChildID:    req.From.ID,
		ParentType: parentType,
		ParentID:   req.To.ID,
	}

	revision, err := h.engine.GetDB().DeleteRelationship(r.Context(), tenantID, edge)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Relationship not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := RelationshipResponse{
		Relationship: Relationship{
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
func MapNodeTypeToUI(nt postgres.NodeType) string {
	switch nt {
	case postgres.NodeUser:
		return "subject"
	case postgres.NodeUA:
		return "subject-set"
	case postgres.NodeObject:
		return "object"
	case postgres.NodeOA:
		return "object-set"
	default:
		return string(nt)
	}
}

func InferRelationshipKind(childType, parentType postgres.NodeType) string {
	if childType == postgres.NodeUser && parentType == postgres.NodeUA {
		return "subject_member_of_set"
	}
	if childType == postgres.NodeUA && parentType == postgres.NodeUA {
		return "subject_set_parent_of_set"
	}
	if childType == postgres.NodeObject && parentType == postgres.NodeOA {
		return "object_member_of_set"
	}
	if childType == postgres.NodeOA && parentType == postgres.NodeOA {
		return "object_set_parent_of_set"
	}
	return "unknown"
}

