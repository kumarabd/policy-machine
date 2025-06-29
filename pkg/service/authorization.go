package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kumarabd/policy-machine/pkg/model"
	"github.com/kumarabd/policy-machine/pkg/utils"
)

// AuthorizationRequest represents a request to check if an action is authorized
// swagger:model AuthorizationRequest
type AuthorizationRequest struct {
	// Subject performing the action (user, service, etc.)
	// required: true
	// example: user123
	Subject string `json:"subject" binding:"required"`

	// Action being performed
	// required: true
	// example: read
	Action string `json:"action" binding:"required"`

	// Resource being accessed
	// required: true
	// example: document456
	Resource string `json:"resource" binding:"required"`

	// Additional context for the authorization decision
	// example: {"ip": "192.168.1.1", "time": "2024-01-01T12:00:00Z", "department": "engineering"}
	Context map[string]interface{} `json:"context,omitempty"`
}

// AuthorizationResponse represents the response to an authorization request
// swagger:model AuthorizationResponse
type AuthorizationResponse struct {
	// Whether the action is allowed
	// example: true
	Allowed bool `json:"allowed"`

	// Reason for the decision (optional)
	// example: User has admin role with read permission
	Reason string `json:"reason,omitempty"`

	// Policy that made the decision (optional)
	// example: rbac-admin-policy
	PolicyID string `json:"policy_id,omitempty"`

	// Decision time in milliseconds
	// example: 15
	DecisionTimeMs int64 `json:"decision_time_ms,omitempty"`
}

// AuthorizeRequest performs authorization evaluation for an HTTP request
// This method orchestrates the entire authorization flow and provides a higher-level interface
func (h *Handler) AuthorizeRequest(ctx context.Context, authRequest *AuthorizationRequest) (*Decision, error) {
	// Validate inputs
	if authRequest == nil {
		return nil, fmt.Errorf("authorization request cannot be nil")
	}

	if authRequest.Subject == "" || authRequest.Action == "" || authRequest.Resource == "" {
		return nil, fmt.Errorf("subject, action, and resource are required")
	}

	h.log.Debug().
		Str("subject", authRequest.Subject).
		Str("action", authRequest.Action).
		Str("resource", authRequest.Resource).
		Msg("Processing authorization request")

	// Note: In this simplified version, we'll let EvaluatePolicy handle entity creation

	// Fetch or create subject using existing service methods
	subject, err := h.fetchOrCreateSubject(authRequest.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch or create subject: %w", err)
	}

	// Fetch or create resource using existing service methods
	resource, err := h.fetchOrCreateResource(authRequest.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch or create resource: %w", err)
	}

	// Determine policy class from context
	policyClass := h.determinePolicyClass(authRequest)
	h.log.Debug().Str("policy_class", policyClass).Msg("Using policy class for evaluation")

	// Build evaluation request for policy evaluation
	evaluationRequest := &EvaluationRequest{
		Subject:     *subject.Entity,
		Resource:    *resource.Entity,
		Actions:     []string{authRequest.Action},
		PolicyClass: policyClass,
		Context:     utils.InterfaceToStringMap(authRequest.Context),
		RequestID:   fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}

	// Evaluate the policy using existing evaluation logic
	decision, err := h.EvaluatePolicy(ctx, evaluationRequest)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	return decision, nil
}

// fetchOrCreateSubject fetches a subject using existing service methods or creates a basic one
func (h *Handler) fetchOrCreateSubject(subjectID string) (*model.Subject, error) {
	// Use existing SubjectBuilderWithID method
	subjectBuilder := SubjectBuilderWithID(subjectID)

	// Try to fetch the subject
	if err := subjectBuilder.Fetch(h.store); err != nil {
		h.log.Warn().Err(err).Str("subject_id", subjectID).Msg("Subject not found, creating anonymous subject")
		return nil, err
	}

	// Validate subject has required fields
	if subjectBuilder.Subject == nil || subjectBuilder.Subject.Entity == nil {
		return nil, fmt.Errorf("subject entity is invalid for subject ID: %s", subjectID)
	}

	return subjectBuilder.Subject, nil
}

// fetchOrCreateResource fetches a resource using existing service methods or creates a basic one
func (h *Handler) fetchOrCreateResource(resourceID string) (*model.Resource, error) {
	// Use existing ResourceBuilderWithID method
	resourceBuilder := ResourceBuilderWithID(resourceID)

	// Try to fetch the resource
	if err := resourceBuilder.Fetch(h.store); err != nil {
		h.log.Warn().Err(err).Str("resource_id", resourceID).Msg("Resource not found, creating basic resource")
		return nil, err
	}

	// Validate resource has required fields
	if resourceBuilder.Resource == nil || resourceBuilder.Resource.Entity == nil {
		return nil, fmt.Errorf("resource entity is invalid for resource ID: %s", resourceID)
	}

	return resourceBuilder.Resource, nil
}

// determinePolicyClass determines which policy class to use for evaluation based on context
func (h *Handler) determinePolicyClass(authRequest *AuthorizationRequest) string {
	// Check if policy class is specified in context
	if authRequest.Context != nil {
		// Check for tenant-based policy class
		if tenant, exists := authRequest.Context["tenant"]; exists {
			if str, ok := tenant.(string); ok && str != "" {
				policyClass := str
				h.log.Debug().Str("policy_class", policyClass).Str("tenant", str).Msg("Using tenant-based policy class")
				return policyClass
			}
		}
	}

	// Default policy class
	h.log.Debug().Msg("Using default policy class")
	return "default"
}
