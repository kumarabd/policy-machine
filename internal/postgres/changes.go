package postgres

import (
	"time"

	"gorm.io/datatypes"
)

type PolicyRevision struct {
	TenantID  string    `gorm:"type:text;primaryKey"`
	Revision  int64     `gorm:"not null;default:0"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type ChangeOp string

const (
	OpAdd    ChangeOp = "ADD"
	OpRemove ChangeOp = "REMOVE"
	OpUpdate ChangeOp = "UPDATE"
)

// Keep this tight and explicit. Payload is "what to apply".
// This is an outbox table: append-only, ordered by Seq.
type PolicyChange struct {
	Seq      int64  `gorm:"primaryKey;autoIncrement"`
	TenantID string `gorm:"type:text;not null;index:idx_changes_tenant_seq,priority:1"`
	Revision int64  `gorm:"not null;index:idx_changes_tenant_rev,priority:2"`

	Kind string   `gorm:"type:text;not null"` // e.g. "ASSIGNMENT_EDGE", "ASSOC_OP", "PROHIB_OP"
	Op   ChangeOp `gorm:"type:text;not null"` // ADD/REMOVE/UPDATE

	Payload datatypes.JSON `gorm:"type:jsonb;not null"` // JSON with the fields needed to apply

	CreatedAt time.Time `gorm:"autoCreateTime"`

	// Helpful composite index: (tenant_id, seq)
	// Add via migration SQL if you want strict ordering perf.
}
