package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/pkg/api"
	"gorm.io/gorm"
)

// ListRules returns paginated list of rules (associations)
func (s *Server) ListRules(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
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
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	rules := make([]api.Rule, len(results))
	for i, res := range results {
		ops, _ := res["operations"].([]string)

		// Get subject type and ID - must be present in response
		subjectType, ok := res["subject_type"].(string)
		if !ok {
			httputil.RespondError(w, http.StatusInternalServerError, "DATA_ERROR", "Missing subject_type in rule response")
			return
		}
		subjectID, ok := res["subject_id"].(uuid.UUID)
		if !ok {
			httputil.RespondError(w, http.StatusInternalServerError, "DATA_ERROR", "Missing subject_id in rule response")
			return
		}

		// Get object type and ID - must be present in response
		objectType, ok := res["object_type"].(string)
		if !ok {
			httputil.RespondError(w, http.StatusInternalServerError, "DATA_ERROR", "Missing object_type in rule response")
			return
		}
		objectID, ok := res["object_id"].(uuid.UUID)
		if !ok {
			httputil.RespondError(w, http.StatusInternalServerError, "DATA_ERROR", "Missing object_id in rule response")
			return
		}

		rules[i] = api.Rule{
			ID:          res["id"].(uuid.UUID),
			Description: "",
			Actions:     ops,
			SubjectSelector: api.NodeRef{
				Type: subjectType,
				ID:   subjectID,
			},
			ObjectSelector: api.NodeRef{
				Type: objectType,
				ID:   objectID,
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
func (s *Server) CreateRule(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Validate subject selector type
	// Note: "subject-set" is API terminology only for the /api/v1/subject-sets endpoint.
	// In rules API, we use "subject-attribute" to refer to SubjectAttribute (UA) entities.
	if req.SubjectSelector.Type != "subject" && req.SubjectSelector.Type != "subject-attribute" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "subject_selector.type must be 'subject' or 'subject-attribute'")
		return
	}
	// Validate object selector type
	// Note: "object-set" is API terminology only for the /api/v1/object-sets endpoint.
	// In rules API, we use "object-attribute" to refer to ObjectAttribute (OA) entities.
	if req.ObjectSelector.Type != "object" && req.ObjectSelector.Type != "object-attribute" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "object_selector.type must be 'object' or 'object-attribute'")
		return
	}
	if len(req.Actions) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "actions are required")
		return
	}

	assocID, revision, err := s.engine.GetDB().CreateRule(r.Context(), tenantID, req.SubjectSelector.Type, req.SubjectSelector.ID, req.ObjectSelector.Type, req.ObjectSelector.ID, req.Actions)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	now := time.Now()
	response := api.RuleResponse{
		Rule: api.Rule{
			ID:              assocID,
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
func (s *Server) GetRule(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	assoc, ops, err := s.engine.GetDB().GetRule(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	subjectType, subjectID := assoc.GetSubjectType()
	objectType, objectID := assoc.GetObjectType()

	rule := api.Rule{
		ID:          assoc.ID,
		Description: "",
		Actions:     ops,
		SubjectSelector: api.NodeRef{
			Type: subjectType,
			ID:   subjectID,
		},
		ObjectSelector: api.NodeRef{
			Type: objectType,
			ID:   objectID,
		},
		Effect:    "ALLOW",
		Enabled:   true,
		CreatedAt: assoc.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// UpdateRule updates a rule
func (s *Server) UpdateRule(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	var req api.UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
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
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	assoc, ops, _ := s.engine.GetDB().GetRule(r.Context(), tenantID, id)
	now := time.Now()

	subjectType, subjectID := assoc.GetSubjectType()
	objectType, objectID := assoc.GetObjectType()

	response := api.RuleResponse{
		Rule: api.Rule{
			ID:          assoc.ID,
			Description: "",
			Actions:     ops,
			SubjectSelector: api.NodeRef{
				Type: subjectType,
				ID:   subjectID,
			},
			ObjectSelector: api.NodeRef{
				Type: objectType,
				ID:   objectID,
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
func (s *Server) DeleteRule(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	_, err = s.engine.GetDB().DeleteRule(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
