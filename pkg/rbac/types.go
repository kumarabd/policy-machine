package rbac

// RBACSubject represents a subject in RBAC space.
// It maps to a policy-machine Subject under the hood.
type Subject struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name"`              // stable identifier
	Kind     string            `json:"kind,omitempty"`    // e.g. "user", "serviceaccount"
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Permission represents a role permission: an action on an object attribute.
type Permission struct {
	Action          string `json:"action"`          // e.g. "read", "write"
	ObjectAttribute string `json:"objectAttribute"` // logical attribute name
}

// Role is backed by a custom SubjectAttribute (subject set) and associations.
type Role struct {
	ID          string       `json:"id,omitempty"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Binding assigns one role to many subjects.
type Binding struct {
	RoleID     string   `json:"roleId"`     // role (subject-attribute) ID
	SubjectIDs []string `json:"subjectIds"` // subject IDs
}

// BindingByName assigns a role (by name) to many subjects (by name).
// Used by the upsert bindings API for seeding/import.
type BindingByName struct {
	RoleName     string   `json:"roleName"`
	SubjectNames []string `json:"subjectNames"`
}

// ObjectAttribute represents a logical attribute for objects.
type ObjectAttribute struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Object represents an RBAC-level object with attributes.
// Attributes are represented as key -> value pairs (e.g. "kind" -> "Pod").
type Object struct {
	ID         string            `json:"id,omitempty"`
	Name       string            `json:"name"`                 // stable identifier
	Kind       string            `json:"kind,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"` // key -> value
}

// UpsertObjectsRequest is used to upsert a batch of objects + attributes.
type UpsertObjectsRequest struct {
	Objects []Object `json:"objects"`
}

// UpsertSubjectsRequest is used to upsert a batch of subjects.
type UpsertSubjectsRequest struct {
	Subjects []Subject `json:"subjects"`
}

// UpsertRolesRequest is used to upsert a batch of roles.
type UpsertRolesRequest struct {
	Roles []Role `json:"roles"`
}

// UpsertBindingsRequest is used to upsert a batch of bindings by logical names.
type UpsertBindingsRequest struct {
	Bindings []BindingByName `json:"bindings"`
}

