package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/validate"
	"gorm.io/gorm"
)

// WithPolicyWriteTx executes a function within a transaction that:
// 1. Runs the provided function
// 2. Bumps the policy revision
// 3. Appends policy changes
// 4. Returns the new revision
func (h *Handler) WithPolicyWriteTx(ctx context.Context, tenantID uuid.UUID, fn func(tx *gorm.DB) error) (int64, error) {
	var revision int64
	err := h.H.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Execute the provided function
		if err := fn(tx); err != nil {
			return err
		}

		// Bump revision
		rev, err := BumpRevision(tx, tenantID)
		if err != nil {
			return err
		}
		revision = rev

		return nil
	})
	return revision, err
}

// AppendPolicyChange appends a change to the policy_changes table
// Should be called within a transaction
func AppendPolicyChange(tx *gorm.DB, tenantID uuid.UUID, revision int64, kind string, op ChangeOp, payload any) error {
	return AppendChange(tx, tenantID, revision, kind, op, payload)
}

// CreateSubject creates a user (subject) and records the change
func (h *Handler) CreateSubject(ctx context.Context, tenantID uuid.UUID, user *User) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		user.TenantID = tenantID
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// Record change
		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"user_id": user.ID,
			"external_id": user.ExternalID,
		}
		return AppendChange(tx, tenantID, rev, "USER_CREATE", OpAdd, payload)
	})
}

// CreateSubjectGroup creates a user attribute (subject group) and records the change
func (h *Handler) CreateSubjectGroup(ctx context.Context, tenantID uuid.UUID, ua *UserAttribute) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		ua.TenantID = tenantID
		if err := tx.Create(ua).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": ua.ID,
			"name": ua.Name,
		}
		return AppendChange(tx, tenantID, rev, "UA_CREATE", OpAdd, payload)
	})
}

// CreateObject creates an object and records the change
func (h *Handler) CreateObject(ctx context.Context, tenantID uuid.UUID, obj *Object) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		obj.TenantID = tenantID
		if err := tx.Create(obj).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"object_id": obj.ID,
			"external_id": obj.ExternalID,
		}
		return AppendChange(tx, tenantID, rev, "OBJECT_CREATE", OpAdd, payload)
	})
}

// CreateObjectGroup creates an object attribute (object group) and records the change
func (h *Handler) CreateObjectGroup(ctx context.Context, tenantID uuid.UUID, oa *ObjectAttribute) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		oa.TenantID = tenantID
		if err := tx.Create(oa).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"oa_id": oa.ID,
			"name": oa.Name,
		}
		return AppendChange(tx, tenantID, rev, "OA_CREATE", OpAdd, payload)
	})
}

// CreateRelationship creates an assignment edge (relationship) and records the change
func (h *Handler) CreateRelationship(ctx context.Context, tenantID uuid.UUID, edge *AssignmentEdge) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		// Use existing AddAssignmentEdge which handles validation and change logging
		// But we need to call it within our transaction
		edge.TenantID = tenantID
		
		// Validate
		if err := validate.ValidateAssignmentEdgeCreate(ctx, tx, tenantID, edge); err != nil {
			return err
		}

		// Check if exists
		var existing AssignmentEdge
		err := tx.Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
			tenantID, edge.ChildType, edge.ChildID, edge.ParentType, edge.ParentID).
			First(&existing).Error
		if err == nil {
			// Already exists - idempotent
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		// Create
		if err := tx.Create(edge).Error; err != nil {
			return err
		}

		// Change will be logged by BumpRevision in WithPolicyWriteTx
		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"child_type":  edge.ChildType,
			"child_id":    edge.ChildID,
			"parent_type": edge.ParentType,
			"parent_id":   edge.ParentID,
		}
		return AppendChange(tx, tenantID, rev, "ASSIGNMENT_EDGE", OpAdd, payload)
	})
}

// DeleteRelationship deletes an assignment edge and records the change
func (h *Handler) DeleteRelationship(ctx context.Context, tenantID uuid.UUID, edge *AssignmentEdge) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		// Find and delete
		result := tx.Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
			tenantID, edge.ChildType, edge.ChildID, edge.ParentType, edge.ParentID).
			Delete(&AssignmentEdge{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"child_type":  edge.ChildType,
			"child_id":    edge.ChildID,
			"parent_type": edge.ParentType,
			"parent_id":   edge.ParentID,
		}
		return AppendChange(tx, tenantID, rev, "ASSIGNMENT_EDGE", OpRemove, payload)
	})
}

// CreateRule creates an association with operations and records the change
func (h *Handler) CreateRule(ctx context.Context, tenantID uuid.UUID, uaID, oaID uuid.UUID, ops []string) (uuid.UUID, int64, error) {
	var assocID uuid.UUID
	revision, err := h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		assoc := Association{
			TenantID:          tenantID,
			UserAttributeID:   uaID,
			ObjectAttributeID: oaID,
		}
		if err := tx.Where("tenant_id = ? AND user_attribute_id = ? AND object_attribute_id = ?",
			tenantID, uaID, oaID).FirstOrCreate(&assoc).Error; err != nil {
			return err
		}
		assocID = assoc.ID

		for _, op := range ops {
			ao := AssociationOperation{
				TenantID:      tenantID,
				AssociationID: assoc.ID,
				Operation:     op,
			}
			if err := tx.Where("tenant_id = ? AND association_id = ? AND operation = ?",
				tenantID, assoc.ID, op).FirstOrCreate(&ao).Error; err != nil {
				return err
			}
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": uaID,
			"oa_id": oaID,
			"ops": ops,
		}
		return AppendChange(tx, tenantID, rev, "ASSOC_OP", OpAdd, payload)
	})
	return assocID, revision, err
}

// DeleteRule deletes an association and records the change
func (h *Handler) DeleteRule(ctx context.Context, tenantID uuid.UUID, assocID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var assoc Association
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, assocID).First(&assoc).Error; err != nil {
			return err
		}

		// Get operations before delete
		var ops []AssociationOperation
		tx.Where("tenant_id = ? AND association_id = ?", tenantID, assocID).Find(&ops)
		operations := make([]string, len(ops))
		for i, op := range ops {
			operations[i] = op.Operation
		}

		// Delete operations
		if err := tx.Where("tenant_id = ? AND association_id = ?", tenantID, assocID).
			Delete(&AssociationOperation{}).Error; err != nil {
			return err
		}

		// Delete association
		if err := tx.Delete(&assoc).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": assoc.UserAttributeID,
			"oa_id": assoc.ObjectAttributeID,
			"ops": operations,
		}
		return AppendChange(tx, tenantID, rev, "ASSOC_OP", OpRemove, payload)
	})
}

// CreateDeny creates a prohibition with operations and records the change
func (h *Handler) CreateDeny(ctx context.Context, tenantID uuid.UUID, subjectType ProhibitionSubjectType, subjectID uuid.UUID, oaID uuid.UUID, ops []string) (uuid.UUID, int64, error) {
	var prohID uuid.UUID
	revision, err := h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		proh := Prohibition{
			TenantID:          tenantID,
			SubjectType:       subjectType,
			SubjectID:         subjectID,
			ObjectAttributeID: oaID,
		}
		if err := tx.Where("tenant_id = ? AND subject_type = ? AND subject_id = ? AND object_attribute_id = ?",
			tenantID, subjectType, subjectID, oaID).FirstOrCreate(&proh).Error; err != nil {
			return err
		}
		prohID = proh.ID

		for _, op := range ops {
			po := ProhibitionOperation{
				TenantID:      tenantID,
				ProhibitionID: proh.ID,
				Operation:     op,
			}
			if err := tx.Where("tenant_id = ? AND prohibition_id = ? AND operation = ?",
				tenantID, proh.ID, op).FirstOrCreate(&po).Error; err != nil {
				return err
			}
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"subject_type": subjectType,
			"subject_id":   subjectID,
			"oa_id":        oaID,
			"op":           ops,
		}
		return AppendChange(tx, tenantID, rev, "PROHIB_OP", OpAdd, payload)
	})
	return prohID, revision, err
}

// DeleteDeny deletes a prohibition and records the change
func (h *Handler) DeleteDeny(ctx context.Context, tenantID uuid.UUID, prohID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var proh Prohibition
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, prohID).First(&proh).Error; err != nil {
			return err
		}

		// Get operations before delete
		var ops []ProhibitionOperation
		tx.Where("tenant_id = ? AND prohibition_id = ?", tenantID, prohID).Find(&ops)
		operations := make([]string, len(ops))
		for i, op := range ops {
			operations[i] = op.Operation
		}

		// Delete operations
		if err := tx.Where("tenant_id = ? AND prohibition_id = ?", tenantID, prohID).
			Delete(&ProhibitionOperation{}).Error; err != nil {
			return err
		}

		// Delete prohibition
		if err := tx.Delete(&proh).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"subject_type": proh.SubjectType,
			"subject_id":   proh.SubjectID,
			"oa_id":        proh.ObjectAttributeID,
			"op":           operations,
		}
		return AppendChange(tx, tenantID, rev, "PROHIB_OP", OpRemove, payload)
	})
}

// GetCurrentRevision returns the current policy revision for a tenant
func (h *Handler) GetCurrentRevision(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var rev PolicyRevision
	if err := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&rev).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil // No revisions yet
		}
		return 0, err
	}
	return rev.Revision, nil
}

// GetPolicyChanges returns policy changes for a tenant after a given sequence
func (h *Handler) GetPolicyChanges(ctx context.Context, tenantID uuid.UUID, afterSeq int64, limit int) ([]PolicyChange, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100 // Default limit
	}
	var changes []PolicyChange
	err := h.H.WithContext(ctx).
		Where("tenant_id = ? AND seq > ?", tenantID, afterSeq).
		Order("seq ASC").
		Limit(limit).
		Find(&changes).Error
	return changes, err
}

