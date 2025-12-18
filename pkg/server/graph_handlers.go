package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/mock"
)

// GetGraphSummary returns counts of all entities
func (h *BaseServer) GetGraphSummary(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetGraphSummary(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	summary, err := h.engine.GetDB().GetGraphSummary(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := GraphSummaryResponse{
		Subjects:      int(summary["subjects"]),
		Objects:       int(summary["objects"]),
		SubjectSets: int(summary["subject_sets"]),
		ObjectSets:  int(summary["object_sets"]),
		Relationships: int(summary["relationships"]),
		Rules:         int(summary["rules"]),
		Denies:        int(summary["denies"]),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGraphNeighborhood returns nodes and edges around a given node
func (h *BaseServer) GetGraphNeighborhood(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetGraphNeighborhood(w, r)
		return
	}

	_, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	nodeType := r.URL.Query().Get("node_type")
	nodeIDStr := r.URL.Query().Get("node_id")
	_ = 1 // depth (for future implementation)
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if _, err := strconv.Atoi(depthStr); err == nil {
			// depth = d (for future use)
		}
	}

	if nodeIDStr == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "node_id is required")
		return
	}

	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid node ID")
		return
	}

	// For now, return empty neighborhood (can be enhanced later)
	response := GraphNeighborhoodResponse{
		Nodes: []GraphNode{
			{
				ID:   nodeID,
				Type: nodeType,
			},
		},
		Edges: []GraphEdge{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GraphSearch searches for nodes by query
func (h *BaseServer) GraphSearch(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GraphSearch(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	query := r.URL.Query().Get("q")
	typesStr := r.URL.Query().Get("types")
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	var types []string
	if typesStr != "" {
		types = strings.Split(typesStr, ",")
	}

	results, err := h.engine.GetDB().GraphSearch(r.Context(), tenantID, query, types, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	nodes := make([]GraphNode, len(results))
	for i, res := range results {
		nodes[i] = GraphNode{
			ID:   res["id"].(uuid.UUID),
			Type: res["type"].(string),
			Name: res["name"].(string),
		}
	}

	response := GraphSearchResponse{
		Nodes: nodes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

