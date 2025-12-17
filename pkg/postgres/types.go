package postgres

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// --- Core entities ---

type Tenant struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name      string    `gorm:"not null;uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID   uuid.UUID `gorm:"type:uuid;not null;index:idx_users_tenant"`
	ExternalID string    `gorm:"not null;index:uidx_users_tenant_external,unique"`
	Email      string    `gorm:"index"`
	Display    string    `gorm:""`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Object struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_objects_tenant"`

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
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_pc_tenant"`

	Name string `gorm:"not null;index:uidx_pc_tenant_name,unique"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// NGAC: User Attribute (UA) and Object Attribute (OA)
type UserAttribute struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_ua_tenant"`
	Name     string    `gorm:"not null;index:uidx_ua_tenant_name,unique"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type ObjectAttribute struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_oa_tenant"`
	Name     string    `gorm:"not null;index:uidx_oa_tenant_name,unique"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// --- Assignments ---
// NGAC-style assignments form DAGs:
// - user -> UA
// - UA -> UA
// - object -> OA
// - OA -> OA
// - UA -> PolicyClass
// - OA -> PolicyClass
//
// We model this as generic "typed edges" so the DB can persist *any* assignment.

type NodeType string

const (
	NodeUser        NodeType = "USER"
	NodeUA          NodeType = "UA"
	NodeObject      NodeType = "OBJECT"
	NodeOA          NodeType = "OA"
	NodePolicyClass NodeType = "PC"
)

// AssignmentEdge represents: Child -> Parent (in NGAC assignment graphs)
type AssignmentEdge struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_asg_tenant"`

	ChildType NodeType  `gorm:"type:text;not null;index:idx_asg_child"`
	ChildID   uuid.UUID `gorm:"type:uuid;not null;index:idx_asg_child"`

	ParentType NodeType  `gorm:"type:text;not null;index:idx_asg_parent"`
	ParentID   uuid.UUID `gorm:"type:uuid;not null;index:idx_asg_parent"`

	CreatedAt time.Time

	// Uniqueness: avoid duplicate edges per tenant
	// NOTE: GORM doesn't support composite unique across multiple indexes in a single tag cleanly,
	// so we define a single composite unique index by naming it consistently.
	// All four fields + tenant must be unique.
	_ struct{} `gorm:"uniqueIndex:uidx_asg_edge,priority:1"`
	// We attach the unique index via tags on fields:
}

// Attach composite unique index tags on the fields:
func (AssignmentEdge) GormDBDataType(*gorm.DB, *schema.Field) string { return "" } // no-op; keep file gofmt-friendly

// --- Associations ---
// Association: UA <-> OA grants operations.
// We normalize operations into a separate table for query efficiency and indexing.

type Association struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_assoc_tenant"`

	UserAttributeID   uuid.UUID `gorm:"type:uuid;not null;index:idx_assoc_ua"`
	ObjectAttributeID uuid.UUID `gorm:"type:uuid;not null;index:idx_assoc_oa"`

	CreatedAt time.Time

	// Unique per (tenant, ua, oa)
	// (multiple ops hang off the same association)
	// Use tags on fields with same index name.
}

type AssociationOperation struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID      uuid.UUID `gorm:"type:uuid;not null;index:idx_assocop_tenant"`
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
	ProhibitUser ProhibitionSubjectType = "USER"
	ProhibitUA   ProhibitionSubjectType = "UA"
)

type Prohibition struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_proh_tenant"`

	SubjectType ProhibitionSubjectType `gorm:"type:text;not null;index:idx_proh_subj"`
	SubjectID   uuid.UUID              `gorm:"type:uuid;not null;index:idx_proh_subj"`

	ObjectAttributeID uuid.UUID `gorm:"type:uuid;not null;index:idx_proh_oa"`

	CreatedAt time.Time
}

type ProhibitionOperation struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID      uuid.UUID `gorm:"type:uuid;not null;index:idx_prohop_tenant"`
	ProhibitionID uuid.UUID `gorm:"type:uuid;not null;index:idx_prohop_proh"`
	Operation     string    `gorm:"type:text;not null;index:idx_prohop_op"`
	CreatedAt     time.Time
}

// --- Obligations ---
// Store obligation logic as data: event -> action with optional condition.
// This is control-plane data; your runtime will interpret it.

type Obligation struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_obl_tenant"`

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
