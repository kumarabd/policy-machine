package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kumarabd/policy-machine/pkg/service"
)

// Unified Policy Management Handlers

// AuthorizeHandler is the main authorization endpoint that works across all access control models
// @Summary Authorize an action
// @Description Check if a subject is authorized to perform an action on a resource
// @Tags 0-core-authorization
// @Accept json
// @Produce json
// @Param request body AuthorizationRequest true "Authorization request"
// @Success 200 {object} AuthorizationResponse "Authorization decision"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/authorize [post]
func (h *BaseServer) AuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	start := time.Now()

	// Parse request body
	var authRequest service.AuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&authRequest); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode authorization request")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON in request body",
		})
		return
	}

	// Validate required fields
	if authRequest.Subject == "" || authRequest.Action == "" || authRequest.Resource == "" {
		h.log.Warn().
			Str("subject", authRequest.Subject).
			Str("action", authRequest.Action).
			Str("resource", authRequest.Resource).
			Msg("Authorization request missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "validation_error",
			Message: "Subject, action, and resource are required",
		})
		return
	}

	h.log.Debug().
		Str("subject", authRequest.Subject).
		Str("action", authRequest.Action).
		Str("resource", authRequest.Resource).
		Msg("Processing authorization request")

	// Create evaluator and perform authorization evaluation using service layer
	decision, err := h.service.AuthorizeRequest(r.Context(), &authRequest)
	if err != nil {
		h.log.Error().Err(err).Msg("Authorization evaluation failed")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "evaluation_error",
			Message: "Failed to evaluate authorization request",
		})
		return
	}

	decisionTime := time.Since(start).Milliseconds()

	response := service.AuthorizationResponse{
		Allowed:        decision.Permit,
		Reason:         decision.Reason,
		PolicyID:       h.extractPolicyID(decision),
		DecisionTimeMs: decisionTime,
	}

	h.log.Info().
		Bool("allowed", response.Allowed).
		Str("subject", authRequest.Subject).
		Str("action", authRequest.Action).
		Str("resource", authRequest.Resource).
		Int64("decision_time_ms", decisionTime).
		Msg("Authorization decision completed")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// extractPolicyID extracts a policy identifier from the decision for audit purposes
func (h *BaseServer) extractPolicyID(decision *service.Decision) string {
	if decision == nil || len(decision.PolicyPath) == 0 {
		return ""
	}

	// Use the last entity in the policy path as a simple policy identifier
	// In a more sophisticated implementation, this might extract actual policy IDs
	lastEntity := decision.PolicyPath[len(decision.PolicyPath)-1]
	if lastEntity != nil {
		return lastEntity.HashID
	}

	return ""
}

// CreatePolicyHandler creates a new policy
// @Summary Create a new policy
// @Description Create a new policy that can support multiple access control models
// @Tags 1-core-policies
// @Accept json
// @Produce json
// @Param policy body CreatePolicyRequest true "Policy creation request"
// @Success 201 {object} CreatePolicyResponse "Policy created successfully"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies [post]
func (h *BaseServer) CreatePolicyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Create policy not implemented"})
}

// GetPoliciesHandler retrieves all policies
// @Summary List all policies
// @Description Get a list of all policies in the system
// @Tags 1-core-policies
// @Produce json
// @Param type query string false "Filter by policy type (rbac, abac, rebac)"
// @Success 200 {array} PolicyDetails "List of policies"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies [get]
func (h *BaseServer) GetPoliciesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Get policies not implemented"})
}

// UpdatePolicyHandler updates an existing policy
// @Summary Update a policy
// @Description Update an existing policy in the system
// @Tags 1-core-policies
// @Accept json
// @Produce json
// @Param id path string true "Policy ID"
// @Param policy body UpdatePolicyRequest true "Policy update request"
// @Success 200 {object} UpdatePolicyResponse "Policy updated successfully"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 404 {object} ErrorResponse "Policy not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies/{id} [put]
func (h *BaseServer) UpdatePolicyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Update policy not implemented"})
}

// DeletePolicyHandler deletes a policy
// @Summary Delete a policy
// @Description Delete an existing policy from the system
// @Tags 1-core-policies
// @Param id path string true "Policy ID"
// @Success 204 "Policy deleted successfully"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 404 {object} ErrorResponse "Policy not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies/{id} [delete]
func (h *BaseServer) DeletePolicyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Delete policy not implemented"})
}

// ValidatePolicyHandler validates a policy
// @Summary Validate a policy
// @Description Validate the syntax and semantics of a policy
// @Tags 1-core-policies
// @Accept json
// @Produce json
// @Param policy body ValidatePolicyRequest true "Policy validation request"
// @Success 200 {object} ValidatePolicyResponse "Policy validation result"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies/validate [post]
func (h *BaseServer) ValidatePolicyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Validate policy not implemented"})
}

// GetPolicyVersionsHandler retrieves all versions of a policy
// @Summary List policy versions
// @Description Get a list of all versions for a specific policy
// @Tags policy-versions
// @Produce json
// @Param id path string true "Policy ID"
// @Success 200 {array} PolicyVersionDetails "List of policy versions"
// @Failure 404 {object} ErrorResponse "Policy not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies/{id}/versions [get]
func (h *BaseServer) GetPolicyVersionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Get policy versions not implemented"})
}

// CreatePolicyVersionHandler creates a new version of a policy
// @Summary Create a new policy version
// @Description Create a new version of an existing policy
// @Tags policy-versions
// @Accept json
// @Produce json
// @Param id path string true "Policy ID"
// @Param version body CreatePolicyVersionRequest true "Policy version creation request"
// @Success 201 {object} CreatePolicyVersionResponse "Policy version created successfully"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 404 {object} ErrorResponse "Policy not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/policies/{id}/versions [post]
func (h *BaseServer) CreatePolicyVersionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not_implemented", "message": "Create policy version not implemented"})
}
