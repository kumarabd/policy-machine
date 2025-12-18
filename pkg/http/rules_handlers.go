package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/mock"
	"gorm.io/gorm"
)

// ListRules returns paginated list of rules (associations)
func (s *HTTP) ListRules(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ListRules(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	filters := make(map[string]interface{})
	if uaIDStr := r.URL.Query().Get("subject_scope_id"); uaIDStr != "" {
		if uaID, err := uuid.Parse(uaIDStr); err == nil {
			filters["subject_scope_id"] = uaID
		}
	}
	if oaIDStr := r.URL.Query().Get("object_scope_id"); oaIDStr != "" {
		if oaID, err := uuid.Parse(oaIDStr); err == nil {
			filters["object_scope_id"] = oaID
		}
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	_, results, nextCursor, hasMore, err := s.engine.GetDB().ListRules(r.Context(), tenantID, filters, limit, cursor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	rules := make([]api.Rule, len(results))
	for i, res := range results {
		ops, _ := res["operations"].([]string)
		uaID := res["ua_id"].(uuid.UUID)
		oaID := res["oa_id"].(uuid.UUID)

		// Generate a name from the scopes if not available
		name := "Rule"
		if desc, ok := res["description"].(string); ok && desc != "" {
			name = desc
		}

		rules[i] = api.Rule{
			ID:          res["id"].(uuid.UUID),
			Name:        name,
			Description: "",
			Actions:     ops,
			SubjectSelector: api.NodeRef{
				Type: "subject-set",
				ID:   uaID,
			},
			ObjectSelector: api.NodeRef{
				Type: "object-set",
				ID:   oaID,
			},
			Effect:  "ALLOW",
			Enabled: true,
		}
		if createdAt, ok := res["created_at"].(time.Time); ok {
			rules[i].CreatedAt = createdAt
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(rules)

	response := api.SearchResponse[api.Rule]{
		Items:      rules,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateRule creates a new rule (association)
func (s *HTTP) CreateRule(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.CreateRule(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.SubjectSelector.Type != "subject-set" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "subject_selector.type must be 'subject-set'")
		return
	}
	if req.ObjectSelector.Type != "object-set" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "object_selector.type must be 'object-set'")
		return
	}
	if len(req.Actions) == 0 {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "actions are required")
		return
	}

	assocID, revision, err := s.engine.GetDB().CreateRule(r.Context(), tenantID, req.SubjectSelector.ID, req.ObjectSelector.ID, req.Actions)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	now := time.Now()
	response := api.RuleResponse{
		Rule: api.Rule{
			ID:              assocID,
			Name:            req.Name,
			Description:     req.Description,
			ScopeID:         req.ScopeID,
			Actions:         req.Actions,
			SubjectSelector: req.SubjectSelector,
			ObjectSelector:  req.ObjectSelector,
			Condition:       req.Condition,
			Effect:          req.Effect,
			Priority:        req.Priority,
			Enabled:         req.Enabled,
			CreatedAt:       now,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetRule returns a rule by ID
func (s *HTTP) GetRule(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.GetRule(w, r)
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
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	assoc, ops, err := s.engine.GetDB().GetRule(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	rule := api.Rule{
		ID:          assoc.ID,
		Name:        "Rule", // TODO: Get from DB if available
		Description: "",
		Actions:     ops,
		SubjectSelector: api.NodeRef{
			Type: "subject-set",
			ID:   assoc.UserAttributeID,
		},
		ObjectSelector: api.NodeRef{
			Type: "object-set",
			ID:   assoc.ObjectAttributeID,
		},
		Effect:    "ALLOW",
		Enabled:   true,
		CreatedAt: assoc.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// UpdateRule updates a rule
func (s *HTTP) UpdateRule(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.UpdateRule(w, r)
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
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	var req api.UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	ops := req.Actions
	if len(ops) == 0 {
		// Get existing operations if not provided
		_, existingOps, _ := s.engine.GetDB().GetRule(r.Context(), tenantID, id)
		ops = existingOps
	}

	revision, err := s.engine.GetDB().UpdateRule(r.Context(), tenantID, id, ops)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	assoc, ops, _ := s.engine.GetDB().GetRule(r.Context(), tenantID, id)
	now := time.Now()

	response := api.RuleResponse{
		Rule: api.Rule{
			ID:          assoc.ID,
			Name:        "Rule", // TODO: Get from DB if available
			Description: "",
			Actions:     ops,
			SubjectSelector: api.NodeRef{
				Type: "subject-set",
				ID:   assoc.UserAttributeID,
			},
			ObjectSelector: api.NodeRef{
				Type: "object-set",
				ID:   assoc.ObjectAttributeID,
			},
			Effect:    "ALLOW",
			Enabled:   true,
			CreatedAt: assoc.CreatedAt,
			UpdatedAt: &now,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteRule deletes a rule
func (s *HTTP) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.DeleteRule(w, r)
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
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	_, err = s.engine.GetDB().DeleteRule(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
