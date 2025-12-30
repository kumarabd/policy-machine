package dataplane

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/mock"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// Authorize handles authorization requests
// @Summary Authorize request
// @Description Checks if a user is allowed to perform an operation on an object
// @Tags authorization
// @Accept json
// @Produce json
// @Param request body http.AuthorizeRequest true "Authorization request"
// @Success 200 {object} http.AuthorizeResponse
// @Router /api/v1/authorize [post]
func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.Authorize(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	allowed, err := s.engine.Decide(r.Context(), req.UserID, req.ObjectID, req.Operation)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DECISION_ERROR", err.Error())
		return
	}

	// Get current revision
	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	response := httputil.AuthorizeResponse{
		Allowed:  allowed,
		Revision: rev,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AuthorizeExplain provides detailed explanation of authorization decision
func (s *Server) AuthorizeExplain(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.AuthorizeExplain(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Get snapshot
	snap := s.engine.Snapshot()
	if snap == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "NO_SNAPSHOT", "Engine snapshot not available")
		return
	}

	// Compute closures (reuse engine logic)
	uaClosure := s.engine.UserUAClosure(snap, req.UserID)
	oaClosure := s.engine.ObjectOAClosure(snap, req.ObjectID)

	// Get allow/deny sets with matches
	allowSet, uaOAMatches := s.engine.AllowedForWithMatches(snap, req.UserID, req.Operation, uaClosure)
	denySet, userDenyMatches, uaDenyMatches := s.engine.DeniedForWithMatches(snap, req.UserID, req.Operation, uaClosure)

	// Convert bitmaps to UUID lists
	subjectClosure := bitmapToUUIDs(uaClosure, snap.UAByIdx())
	objectClosure := bitmapToUUIDs(oaClosure, snap.OAByIdx())

	// Query database to get association IDs that matched
	associations, err := s.engine.GetDB().GetAssociationsByUAOA(r.Context(), tenantID, uaOAMatches, req.Operation)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	allowHits := make([]uuid.UUID, len(associations))
	for i, assoc := range associations {
		allowHits[i] = assoc.ID
	}

	// Query database to get prohibition IDs that matched
	prohibitions, err := s.engine.GetDB().GetProhibitionsBySubjectOA(r.Context(), tenantID, userDenyMatches, uaDenyMatches, req.Operation)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	denyHits := make([]uuid.UUID, len(prohibitions))
	for i, proh := range prohibitions {
		denyHits[i] = proh.ID
	}

	// Compute final decision
	effectiveAllow := roaring.And(allowSet, oaClosure)
	effectiveDeny := roaring.And(denySet, oaClosure)
	allowed := effectiveAllow.GetCardinality() > 0 && effectiveDeny.GetCardinality() == 0

	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	response := httputil.ExplainResponse{
		Allowed:  allowed,
		Revision: rev,
	}
	response.Explain.SubjectClosure = subjectClosure
	response.Explain.ObjectClosure = objectClosure
	response.Explain.AllowHits = allowHits
	response.Explain.DenyHits = denyHits
	response.Explain.EffectiveAllowSetSize = int(effectiveAllow.GetCardinality())
	response.Explain.EffectiveDenySetSize = int(effectiveDeny.GetCardinality())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Evaluate handles evaluate requests (UI-compatible format)
// @Summary Evaluate request
// @Description Evaluates if a subject is allowed to perform an action on an object
// @Tags authorization
// @Accept json
// @Produce json
// @Param request body api.EvaluateRequest true "Evaluation request"
// @Success 200 {object} api.EvaluateResponse
// @Router /api/v1/evaluate [post]
func (s *Server) Evaluate(w http.ResponseWriter, r *http.Request) {
	if httputil.IsMockMode(r.Context()) {
		mock.Evaluate(w, r)
		return
	}

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Parse UUIDs
	subjectID, err := uuid.Parse(req.Subject.ID)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_SUBJECT_ID", "Invalid subject ID format")
		return
	}

	objectID, err := uuid.Parse(req.Object.ID)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_OBJECT_ID", "Invalid object ID format")
		return
	}

	// Get current revision
	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)
	versionID := strconv.FormatInt(rev, 10)

	// If explain is requested, use AuthorizeExplain logic
	if req.Explain {
		// Get snapshot
		snap := s.engine.Snapshot()
		if snap == nil {
			httputil.RespondError(w, http.StatusInternalServerError, "NO_SNAPSHOT", "Engine snapshot not available")
			return
		}

		// Compute closures
		uaClosure := s.engine.UserUAClosure(snap, subjectID)
		oaClosure := s.engine.ObjectOAClosure(snap, objectID)

		// Get allow/deny sets with matches
		allowSet, uaOAMatches := s.engine.AllowedForWithMatches(snap, subjectID, req.Action, uaClosure)
		denySet, userDenyMatches, uaDenyMatches := s.engine.DeniedForWithMatches(snap, subjectID, req.Action, uaClosure)

		// Query database to get association IDs that matched
		associations, err := s.engine.GetDB().GetAssociationsByUAOA(r.Context(), tenantID, uaOAMatches, req.Action)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		allowHits := make([]uuid.UUID, len(associations))
		for i, assoc := range associations {
			allowHits[i] = assoc.ID
		}

		// Query database to get prohibition IDs that matched
		prohibitions, err := s.engine.GetDB().GetProhibitionsBySubjectOA(r.Context(), tenantID, userDenyMatches, uaDenyMatches, req.Action)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		denyHits := make([]uuid.UUID, len(prohibitions))
		for i, proh := range prohibitions {
			denyHits[i] = proh.ID
		}

		// Compute final decision
		effectiveAllow := roaring.And(allowSet, oaClosure)
		effectiveDeny := roaring.And(denySet, oaClosure)
		allowed := effectiveAllow.GetCardinality() > 0 && effectiveDeny.GetCardinality() == 0

		decision := "DENY"
		if allowed {
			decision = "ALLOW"
		}

		// Build trace with matched rules and denies
		trace := &api.ExplainTrace{}
		trace.Summary.MatchedRulesCount = len(allowHits)
		trace.Summary.DenyRulesCount = len(denyHits)
		trace.Summary.EffectiveDecision = decision
		trace.Summary.VersionID = versionID

		// Populate matched rules
		matchedRules := make([]api.MatchedRule, len(associations))
		for i, assoc := range associations {
			matchedRules[i] = api.MatchedRule{
				RuleID:   assoc.ID.String(),
				RuleName: "", // Association doesn't have a name field
				Effect:   "ALLOW",
			}
		}
		trace.MatchedRules = matchedRules

		// Populate deny rules
		denyRules := make([]api.DenyRule, len(prohibitions))
		for i, proh := range prohibitions {
			denyRules[i] = api.DenyRule{
				RuleID:   proh.ID.String(),
				RuleName: "", // Prohibition doesn't have a name field
			}
		}
		trace.Denies = denyRules

		response := api.EvaluateResponse{
			Decision:    decision,
			VersionID:   versionID,
			EvaluatedAt: time.Now(),
			Trace:       trace,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Simple authorization check
	allowed, err := s.engine.Decide(r.Context(), subjectID, objectID, req.Action)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DECISION_ERROR", err.Error())
		return
	}

	decision := "DENY"
	if allowed {
		decision = "ALLOW"
	}

	response := api.EvaluateResponse{
		Decision:    decision,
		VersionID:   versionID,
		EvaluatedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper to convert bitmap to UUID list
func bitmapToUUIDs(bmp *roaring.Bitmap, idArray []uuid.UUID) []uuid.UUID {
	if bmp == nil {
		return []uuid.UUID{}
	}
	result := make([]uuid.UUID, 0, bmp.GetCardinality())
	it := bmp.Iterator()
	for it.HasNext() {
		idx := it.Next()
		if int(idx) < len(idArray) {
			result = append(result, idArray[idx])
		}
	}
	return result
}
