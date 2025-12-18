package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/internal/mock"
)

// Authorize handles authorization requests
// @Summary Authorize request
// @Description Checks if a user is allowed to perform an operation on an object
// @Tags authorization
// @Accept json
// @Produce json
// @Param request body AuthorizeRequest true "Authorization request"
// @Success 200 {object} AuthorizeResponse
// @Router /api/v1/authorize [post]
func (s *HTTP) Authorize(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.Authorize(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	allowed, err := s.engine.Decide(r.Context(), req.UserID, req.ObjectID, req.Operation)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DECISION_ERROR", err.Error())
		return
	}

	// Get current revision
	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	response := AuthorizeResponse{
		Allowed:  allowed,
		Revision: rev,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AuthorizeExplain provides detailed explanation of authorization decision
func (s *HTTP) AuthorizeExplain(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.AuthorizeExplain(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Get snapshot
	snap := s.engine.Snapshot()
	if snap == nil {
		respondError(w, http.StatusInternalServerError, "NO_SNAPSHOT", "Engine snapshot not available")
		return
	}

	// Compute closures (reuse engine logic)
	uaClosure := s.engine.UserUAClosure(snap, req.UserID)
	oaClosure := s.engine.ObjectOAClosure(snap, req.ObjectID)

	// Get allow/deny sets
	allowSet := s.engine.AllowedFor(snap, req.UserID, req.Operation, uaClosure)
	denySet := s.engine.DeniedFor(snap, req.UserID, req.Operation, uaClosure)

	// Convert bitmaps to UUID lists
	subjectClosure := bitmapToUUIDs(uaClosure, snap.UAByIdx())
	objectClosure := bitmapToUUIDs(oaClosure, snap.OAByIdx())

	// For allow/deny hits, we'd need to track which associations/prohibitions matched
	// For now, return empty lists
	allowHits := []uuid.UUID{}
	denyHits := []uuid.UUID{}

	// Compute final decision
	effectiveAllow := roaring.And(allowSet, oaClosure)
	effectiveDeny := roaring.And(denySet, oaClosure)
	allowed := effectiveAllow.GetCardinality() > 0 && effectiveDeny.GetCardinality() == 0

	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	response := ExplainResponse{
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
// @Param request body EvaluateRequest true "Evaluation request"
// @Success 200 {object} EvaluateResponse
// @Router /api/v1/evaluate [post]
func (s *HTTP) Evaluate(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.Evaluate(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Parse UUIDs
	subjectID, err := uuid.Parse(req.Subject.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_SUBJECT_ID", "Invalid subject ID format")
		return
	}

	objectID, err := uuid.Parse(req.Object.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_OBJECT_ID", "Invalid object ID format")
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
			respondError(w, http.StatusInternalServerError, "NO_SNAPSHOT", "Engine snapshot not available")
			return
		}

		// Compute closures
		uaClosure := s.engine.UserUAClosure(snap, subjectID)
		oaClosure := s.engine.ObjectOAClosure(snap, objectID)

		// Get allow/deny sets
		allowSet := s.engine.AllowedFor(snap, subjectID, req.Action, uaClosure)
		denySet := s.engine.DeniedFor(snap, subjectID, req.Action, uaClosure)

		// For allow/deny hits, we'd need to track which associations/prohibitions matched
		allowHits := []uuid.UUID{}
		denyHits := []uuid.UUID{}

		// Compute final decision
		effectiveAllow := roaring.And(allowSet, oaClosure)
		effectiveDeny := roaring.And(denySet, oaClosure)
		allowed := effectiveAllow.GetCardinality() > 0 && effectiveDeny.GetCardinality() == 0

		decision := "DENY"
		if allowed {
			decision = "ALLOW"
		}

		// Build trace
		trace := &api.ExplainTrace{}
		trace.Summary.MatchedRulesCount = len(allowHits)
		trace.Summary.DenyRulesCount = len(denyHits)
		trace.Summary.EffectiveDecision = decision
		trace.Summary.VersionID = versionID
		trace.MatchedRules = []api.MatchedRule{} // TODO: Populate from actual rule matches
		trace.Denies = []api.DenyRule{}          // TODO: Populate from actual deny matches

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
		respondError(w, http.StatusInternalServerError, "DECISION_ERROR", err.Error())
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
