package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/postgres/validate"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func BumpRevision(tx *gorm.DB, tenantID uuid.UUID) (int64, error) {
	rev := PolicyRevision{TenantID: tenantID}

	// Upsert: increment revision atomically
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.Assignments(map[string]any{"revision": gorm.Expr("policy_revisions.revision + 1")}),
	}).Create(&rev).Error; err != nil {
		return 0, err
	}

	// Read back the updated revision
	if err := tx.First(&rev, "tenant_id = ?", tenantID).Error; err != nil {
		return 0, err
	}
	return rev.Revision, nil
}

func AppendChange(tx *gorm.DB, tenantID uuid.UUID, revision int64, kind string, op ChangeOp, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ch := PolicyChange{
		TenantID: tenantID,
		Revision: revision,
		Kind:     kind,
		Op:       op,
		Payload:  b,
	}
	return tx.Create(&ch).Error
}

// AddAssignmentEdge creates an assignment edge with validation
func AddAssignmentEdge(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, edge AssignmentEdge) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Validate before inserting
		if err := validate.ValidateAssignmentEdgeCreate(ctx, tx, tenantID, &edge); err != nil {
			return err
		}

		// Check if edge already exists (idempotent - validation returns nil if exists)
		var existing AssignmentEdge
		err := tx.WithContext(ctx).
			Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
				tenantID, edge.ChildType, edge.ChildID, edge.ParentType, edge.ParentID).
			First(&existing).Error
		if err == nil {
			// Edge already exists - idempotent success, no need to create or emit change
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		// Set tenant ID and create edge
		edge.TenantID = tenantID
		if err := tx.Create(&edge).Error; err != nil {
			return err
		}

		// Bump revision and append to change log
		rev, err := BumpRevision(tx, tenantID)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"child_type":  edge.ChildType,
			"child_id":    edge.ChildID,
			"parent_type": edge.ParentType,
			"parent_id":   edge.ParentID,
		}
		return AppendChange(tx, tenantID, rev, "ASSIGNMENT_EDGE", OpAdd, payload)
	})
}
