package postgres

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// --- Core entities ---

type Tenant struct {
	ID        string `gorm:"type:text;not null;primaryKey"`
	Name      string `gorm:"not null;index:uidx_tenants_name,unique"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Subject struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID   string    `gorm:"type:text;not null;index:idx_subjects_tenant"`
	ExternalID string    `gorm:"not null;index:uidx_subjects_tenant_external,unique"`
	Email      string    `gorm:"index"`
	Display    string    `gorm:""`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// TableName specifies the table name for GORM
func (Subject) TableName() string {
	return "subjects"
}

type Object struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_objects_tenant"`

	// App-level identifier for the protected resource
	ExternalID string `gorm:"not null;index:uidx_objects_tenant_external,unique"`

	// Optional: object type (doc, project, cluster, secret, etc.)
	Type string `gorm:"not null;index:idx_objects_tenant_type"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type PolicyClass struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_pc_tenant"`

	Name string `gorm:"not null;index:uidx_pc_tenant_name,unique"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// NGAC: Subject Attribute (UA) and Object Attribute (OA)
// Subject attributes = Subject sets (same thing)
// Object attributes = Object sets (same thing)
// These can represent:
// - Sets: containers for actual subjects/objects (User, ServiceAccount, Group, or Pod, Deployment, etc.)
// - Attributes: metadata-based groupings (namespace, labels, apigroup, version, etc.)
type SubjectAttribute struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_ua_tenant"`
	Name     string    `gorm:"not null;index:uidx_ua_tenant_name,unique"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// TableName specifies the table name for GORM
func (SubjectAttribute) TableName() string {
	return "subject_attributes"
}

// SubjectSet is an alias for SubjectAttribute (same thing)
type SubjectSet = SubjectAttribute

type ObjectAttribute struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_oa_tenant"`
	Name     string    `gorm:"not null;index:uidx_oa_tenant_name,unique"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// ObjectSet is an alias for ObjectAttribute (same thing)
type ObjectSet = ObjectAttribute

// --- Assignments ---
// NGAC-style assignments form DAGs:
// - subject -> SubjectSet (USER -> SUBJECT_SET)
// - subject -> UA (USER -> UA) for metadata-based grouping
// - SubjectSet -> UA (SUBJECT_SET -> UA) to assign set to attribute
// - UA -> UA (hierarchical attributes)
// - object -> ObjectSet (OBJECT -> OBJECT_SET)
// - object -> OA (OBJECT -> OA) for metadata-based grouping
// - ObjectSet -> OA (OBJECT_SET -> OA) to assign set to attribute
// - OA -> OA (hierarchical attributes)
// - UA -> PolicyClass
// - OA -> PolicyClass
//
// We model this as generic "typed edges" so the DB can persist *any* assignment.

type NodeType string

const (
	NodeSubject     NodeType = "USER"
	NodeUA          NodeType = "UA"
	NodeObject      NodeType = "OBJECT"
	NodeOA          NodeType = "OA"
	NodePolicyClass NodeType = "PC"
)

// AssignmentEdge represents: Child -> Parent (in NGAC assignment graphs)
type AssignmentEdge struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_asg_tenant"`

	ChildType NodeType  `gorm:"type:text;not null;index:idx_asg_child"`
	ChildID   uuid.UUID `gorm:"type:uuid;not null;index:idx_asg_child"`

	ParentType NodeType  `gorm:"type:text;not null;index:idx_asg_parent"`
	ParentID   uuid.UUID `gorm:"type:uuid;not null;index:idx_asg_parent"`

	CreatedAt time.Time

	// Uniqueness: avoid duplicate edges per tenant
	// The composite unique index uidx_asg_edge is defined on:
	// tenant_id, child_type, child_id, parent_type, parent_id
	// This is created via SQL in postgres.go for precision.
	// We don't use GORM uniqueIndex tags here to avoid conflicts with the SQL index.
}

// TableName specifies the table name for GORM
func (AssignmentEdge) TableName() string {
	return "assignment_edges"
}

// Methods for validate package interface
func (e *AssignmentEdge) GetTenantID() string    { return e.TenantID }
func (e *AssignmentEdge) GetChildType() string   { return string(e.ChildType) }
func (e *AssignmentEdge) GetChildID() uuid.UUID  { return e.ChildID }
func (e *AssignmentEdge) GetParentType() string  { return string(e.ParentType) }
func (e *AssignmentEdge) GetParentID() uuid.UUID { return e.ParentID }
func (e *AssignmentEdge) SetTenantID(id string)  { e.TenantID = id }

// --- Associations ---
// Association: UA <-> OA grants operations.
// We normalize operations into a separate table for query efficiency and indexing.

type Association struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_assoc_tenant"`

	SubjectAttributeID uuid.UUID `gorm:"type:uuid;not null;index:idx_assoc_ua"`
	ObjectAttributeID  uuid.UUID `gorm:"type:uuid;not null;index:idx_assoc_oa"`

	CreatedAt time.Time

	// Unique per (tenant, subject_attribute, object_attribute)
	// (multiple ops hang off the same association)
	// Use tags on fields with same index name.
}

type AssociationOperation struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID      string    `gorm:"type:text;not null;index:idx_assocop_tenant"`
	AssociationID uuid.UUID `gorm:"type:uuid;not null;index:idx_assocop_assoc"`

	// Operation name: "read", "write", "delete", "admin", etc.
	Operation string `gorm:"type:text;not null;index:idx_assocop_op"`

	CreatedAt time.Time

	// Unique per (tenant, association, operation)
}

// --- Prohibitions (optional, but common) ---
// A simple, useful model: deny a subject (USER or UA) from an OA for one/more operations.
// You can extend this later (containers, intersections, complements, etc.)

type ProhibitionSubjectType string

const (
	ProhibitSubject ProhibitionSubjectType = "USER"
	ProhibitUA      ProhibitionSubjectType = "UA"
)

type Prohibition struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_proh_tenant"`

	SubjectType ProhibitionSubjectType `gorm:"type:text;not null;index:idx_proh_subj"`
	SubjectID   uuid.UUID              `gorm:"type:uuid;not null;index:idx_proh_subj"`

	ObjectAttributeID uuid.UUID `gorm:"type:uuid;not null;index:idx_proh_oa"`

	CreatedAt time.Time
}

type ProhibitionOperation struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID      string    `gorm:"type:text;not null;index:idx_prohop_tenant"`
	ProhibitionID uuid.UUID `gorm:"type:uuid;not null;index:idx_prohop_proh"`
	Operation     string    `gorm:"type:text;not null;index:idx_prohop_op"`
	CreatedAt     time.Time
}

// --- Obligations ---
// Store obligation logic as data: event -> action with optional condition.
// This is control-plane data; your runtime will interpret it.

type Obligation struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID string    `gorm:"type:text;not null;index:idx_obl_tenant"`

	// Example: "ACCESS_GRANTED", "ACCESS_DENIED", "ASSIGNMENT_ADDED", etc.
	Event string `gorm:"type:text;not null;index:idx_obl_event"`

	// JSON payload specifying action(s) to take: log, notify, mask, ticket, etc.
	Action datatypes.JSON `gorm:"type:jsonb;not null"`

	// Optional condition (JSON) that runtime can evaluate (ABAC-style filter)
	Condition datatypes.JSON `gorm:"type:jsonb"`

	Enabled bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
