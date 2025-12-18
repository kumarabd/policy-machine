package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/mock"
)

// GetGraphSummary returns counts of all entities
func (s *HTTP) GetGraphSummary(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetGraphSummary(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	summary, err := s.engine.GetDB().GetGraphSummary(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := GraphSummaryResponse{
		Subjects:      int(summary["subjects"]),
		Objects:       int(summary["objects"]),
		SubjectSets:   int(summary["subject_sets"]),
		ObjectSets:    int(summary["object_sets"]),
		Relationships: int(summary["relationships"]),
		Rules:         int(summary["rules"]),
		Denies:        int(summary["denies"]),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGraphNeighborhood returns nodes and edges around a given node
// Supports both GET (query params) and POST (request body) for UI compatibility
func (s *HTTP) GetGraphNeighborhood(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetGraphNeighborhood(w, r)
		return
	}

	_, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var nodeType, nodeIDStr string
	var depth int = 1

	// Handle POST request with JSON body (UI format)
	if r.Method == "POST" {
		var reqBody struct {
			Seed struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"seed"`
			Depth int `json:"depth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			nodeType = reqBody.Seed.Type
			nodeIDStr = reqBody.Seed.ID
			if reqBody.Depth > 0 {
				depth = reqBody.Depth
			}
		}
	}

	// Fallback to GET query parameters if POST body parsing failed or it's a GET request
	if nodeIDStr == "" {
		nodeType = r.URL.Query().Get("node_type")
		nodeIDStr = r.URL.Query().Get("node_id")
		if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
			if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
				depth = d
			}
		}
	}

	// depth is reserved for future implementation of neighborhood depth traversal
	_ = depth

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
func (s *HTTP) GraphSearch(w http.ResponseWriter, r *http.Request) {
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

	results, err := s.engine.GetDB().GraphSearch(r.Context(), tenantID, query, types, limit)
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
