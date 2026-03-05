package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"gorm.io/gorm"
)

// ListDenies returns paginated list of deny rules (prohibitions)
func (s *Server) ListDenies(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	filters := make(map[string]interface{})
	if subjectType := r.URL.Query().Get("subject_type"); subjectType != "" {
		filters["subject_type"] = subjectType
	}
	if subjectIDStr := r.URL.Query().Get("subject_id"); subjectIDStr != "" {
		if subjectID, err := uuid.Parse(subjectIDStr); err == nil {
			filters["subject_id"] = subjectID
		}
	}
	if targetIDStr := r.URL.Query().Get("target_id"); targetIDStr != "" {
		if targetID, err := uuid.Parse(targetIDStr); err == nil {
			filters["target_id"] = targetID
		}
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	_, results, nextCursor, hasMore, err := s.engine.GetDB().ListDenies(r.Context(), tenantID, filters, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	denies := make([]httputil.Deny, len(results))
	for i, res := range results {
		ops, _ := res["operations"].([]string)
		subjectType := "subject"
		if res["subject_type"].(postgres.ProhibitionSubjectType) == postgres.ProhibitUA {
			subjectType = "subject-attribute"
		}
		denies[i] = httputil.Deny{
			ID: res["id"].(uuid.UUID),
			Subject: httputil.NodeRef{
				Type: subjectType,
				ID:   res["subject_id"].(uuid.UUID),
			},
			Operations: ops,
			Targets: []httputil.Scope{
				{
					Type: "object-attribute",
					ID:   res["oa_id"].(uuid.UUID),
				},
			},
			CreatedAt: res["created_at"].(time.Time),
		}
	}

	response := httputil.ListDeniesResponse{
		Denies:  denies,
		Cursor:  nextCursor,
		HasMore: hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateDeny creates a new deny rule (prohibition)
func (s *Server) CreateDeny(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.CreateDenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if len(req.Operations) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "operations are required")
		return
	}
	if len(req.Targets) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "targets are required")
		return
	}

	// Map subject type
	// Note: "subject-set" is API terminology only for the /api/v1/subject-sets endpoint.
	// In Deny API, we use "subject-attribute" to refer to SubjectAttribute (UA) entities.
	subjectType := postgres.ProhibitSubject
	if req.Subject.Type == "subject-attribute" {
		subjectType = postgres.ProhibitUA
	}

	// For now, use first target (could support multiple later)
	oaID := req.Targets[0].ID

	prohID, revision, err := s.engine.GetDB().CreateDeny(r.Context(), tenantID, subjectType, req.Subject.ID, oaID, req.Operations)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := httputil.DenyResponse{
		Deny: httputil.Deny{
			ID:          prohID,
			Subject:     req.Subject,
			Operations:  req.Operations,
			Targets:     req.Targets,
			Description: req.Description,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetDeny returns a deny rule by ID
func (s *Server) GetDeny(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid deny ID")
		return
	}

	proh, ops, err := s.engine.GetDB().GetDeny(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Deny rule not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	subjectType := "subject"
	if proh.SubjectType == postgres.ProhibitUA {
		subjectType = "subject-attribute"
	}

	deny := httputil.Deny{
		ID: proh.ID,
		Subject: httputil.NodeRef{
			Type: subjectType,
			ID:   proh.SubjectID,
		},
		Operations: ops,
		Targets: []httputil.Scope{
			{
				Type: "object-attribute",
				ID:   proh.ObjectAttributeID,
			},
		},
		CreatedAt: proh.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deny)
}

// UpdateDeny updates a deny rule
func (s *Server) UpdateDeny(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid deny ID")
		return
	}

	var req httputil.UpdateDenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	ops := req.Operations
	if len(ops) == 0 {
		_, existingOps, _ := s.engine.GetDB().GetDeny(r.Context(), tenantID, id)
		ops = existingOps
	}

	oaIDs := make([]uuid.UUID, 0)
	if len(req.Targets) > 0 {
		for _, t := range req.Targets {
			oaIDs = append(oaIDs, t.ID)
		}
	}

	revision, err := s.engine.GetDB().UpdateDeny(r.Context(), tenantID, id, ops, oaIDs)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Deny rule not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	proh, ops, _ := s.engine.GetDB().GetDeny(r.Context(), tenantID, id)

	subjectType := "subject"
	if proh.SubjectType == postgres.ProhibitUA {
		subjectType = "subject-attribute"
	}

	response := httputil.DenyResponse{
		Deny: httputil.Deny{
			ID: proh.ID,
			Subject: httputil.NodeRef{
				Type: subjectType,
				ID:   proh.SubjectID,
			},
			Operations: ops,
			Targets: []httputil.Scope{
				{
					Type: "object-attribute",
					ID:   proh.ObjectAttributeID,
				},
			},
			CreatedAt: proh.CreatedAt,
		},
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteDeny deletes a deny rule
func (s *Server) DeleteDeny(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid deny ID")
		return
	}

	_, err = s.engine.GetDB().DeleteDeny(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Deny rule not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
