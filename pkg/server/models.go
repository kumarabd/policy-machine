package server

// CreateRoleRequest represents the request body for creating a new role
// swagger:model CreateRoleRequest
type CreateRoleRequest struct {
	// The name of the role
	// required: true
	// example: admin
	Name string `json:"name" binding:"required"`

	// Description of the role
	// example: Administrator role with full access
	Description string `json:"description,omitempty"`

	// Additional properties for the role
	// example: {"department": "IT", "level": "high"}
	Properties map[string]string `json:"properties,omitempty"`
}

// CreateRoleResponse represents the response body for role creation
// swagger:model CreateRoleResponse
type CreateRoleResponse struct {
	// Success message
	// example: Role created successfully
	Message string `json:"message"`

	// Created role details
	Role RoleDetails `json:"role"`
}

// RoleDetails represents detailed information about a role
// swagger:model RoleDetails
type RoleDetails struct {
	// The name of the role
	// example: admin
	Name string `json:"name"`

	// Description of the role
	// example: Administrator role with full access
	Description string `json:"description,omitempty"`

	// Additional properties for the role
	// example: {"department": "IT", "level": "high", "type": "role", "parent": "role"}
	Properties map[string]string `json:"properties"`

	// Unique entity identifier
	// example: role_admin_123abc
	EntityID string `json:"entity_id"`
}

// ErrorResponse represents an error response
// swagger:model ErrorResponse
type ErrorResponse struct {
	// Error code
	// example: validation_error
	Error string `json:"error"`

	// Error message
	// example: Role name is required
	Message string `json:"message"`
}

// Placeholder types for swagger generation (these would be properly implemented in a real system)

// swagger:model CreateABACPolicyRequest
type CreateABACPolicyRequest struct{}

// swagger:model CreateABACPolicyResponse
type CreateABACPolicyResponse struct{}

// swagger:model ABACPolicyDetails
type ABACPolicyDetails struct{}

// swagger:model UpdateABACPolicyRequest
type UpdateABACPolicyRequest struct{}

// swagger:model UpdateABACPolicyResponse
type UpdateABACPolicyResponse struct{}

// swagger:model CreateAttributeDefinitionRequest
type CreateAttributeDefinitionRequest struct{}

// swagger:model CreateAttributeDefinitionResponse
type CreateAttributeDefinitionResponse struct{}

// swagger:model AttributeDefinitionDetails
type AttributeDefinitionDetails struct{}

// swagger:model CreateReBACSchemaRequest
type CreateReBACSchemaRequest struct{}

// swagger:model CreateReBACSchemaResponse
type CreateReBACSchemaResponse struct{}

// swagger:model ReBACSchemaDetails
type ReBACSchemaDetails struct{}

// swagger:model UpdateReBACSchemaRequest
type UpdateReBACSchemaRequest struct{}

// swagger:model UpdateReBACSchemaResponse
type UpdateReBACSchemaResponse struct{}

// swagger:model CreateRelationTypeRequest
type CreateRelationTypeRequest struct{}

// swagger:model CreateRelationTypeResponse
type CreateRelationTypeResponse struct{}

// swagger:model RelationTypeDetails
type RelationTypeDetails struct{}

// swagger:model CreatePolicyRequest
type CreatePolicyRequest struct{}

// swagger:model CreatePolicyResponse
type CreatePolicyResponse struct{}

// swagger:model PolicyDetails
type PolicyDetails struct{}

// swagger:model UpdatePolicyRequest
type UpdatePolicyRequest struct{}

// swagger:model UpdatePolicyResponse
type UpdatePolicyResponse struct{}

// swagger:model ValidatePolicyRequest
type ValidatePolicyRequest struct{}

// swagger:model ValidatePolicyResponse
type ValidatePolicyResponse struct{}

// swagger:model PolicyVersionDetails
type PolicyVersionDetails struct{}

// swagger:model CreatePolicyVersionRequest
type CreatePolicyVersionRequest struct{}

// swagger:model CreatePolicyVersionResponse
type CreatePolicyVersionResponse struct{}

// swagger:model CreatePolicyClassRequest
type CreatePolicyClassRequest struct{}

// swagger:model CreatePolicyClassResponse
type CreatePolicyClassResponse struct{}

// swagger:model PolicyClassDetails
type PolicyClassDetails struct{}

// swagger:model CreateUserAttributeRequest
type CreateUserAttributeRequest struct{}

// swagger:model CreateUserAttributeResponse
type CreateUserAttributeResponse struct{}

// swagger:model CreateObjectAttributeRequest
type CreateObjectAttributeRequest struct{}

// swagger:model CreateObjectAttributeResponse
type CreateObjectAttributeResponse struct{}

// swagger:model CreateAssignmentRequest
type CreateAssignmentRequest struct{}

// swagger:model CreateAssignmentResponse
type CreateAssignmentResponse struct{}

// swagger:model CreateAssociationRequest
type CreateAssociationRequest struct{}

// swagger:model CreateAssociationResponse
type CreateAssociationResponse struct{}

// swagger:model NGACGraphResponse
type NGACGraphResponse struct{}

// RBAC additional types
// swagger:model UpdateRoleRequest
type UpdateRoleRequest struct{}

// swagger:model UpdateRoleResponse
type UpdateRoleResponse struct{}

// swagger:model CreatePermissionRequest
type CreatePermissionRequest struct{}

// swagger:model CreatePermissionResponse
type CreatePermissionResponse struct{}

// swagger:model PermissionDetails
type PermissionDetails struct{}

// swagger:model AssignPermissionRequest
type AssignPermissionRequest struct{}

// swagger:model AssignPermissionResponse
type AssignPermissionResponse struct{}

// swagger:model RemovePermissionRequest
type RemovePermissionRequest struct{}

// swagger:model RemovePermissionResponse
type RemovePermissionResponse struct{}
