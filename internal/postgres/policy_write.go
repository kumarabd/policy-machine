package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/validate"
	"gorm.io/gorm"
)

// WithPolicyWriteTx executes a function within a transaction that:
// 1. Runs the provided function
// 2. Bumps the policy revision
// 3. Appends policy changes
// 4. Returns the new revision
func (h *Handler) WithPolicyWriteTx(ctx context.Context, tenantID string, fn func(tx *gorm.DB) error) (int64, error) {
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
func AppendPolicyChange(tx *gorm.DB, tenantID string, revision int64, kind string, op ChangeOp, payload any) error {
	return AppendChange(tx, tenantID, revision, kind, op, payload)
}

// CreateSubject creates a subject and records the change
// It is idempotent: if a subject with the same tenant_id and external_id exists, it returns the existing subject
func (h *Handler) CreateSubject(ctx context.Context, tenantID string, subject *Subject) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		subject.TenantID = tenantID

		// Check if subject already exists (idempotent)
		var existing Subject
		err := tx.Where("tenant_id = ? AND external_id = ?", tenantID, subject.ExternalID).First(&existing).Error
		if err == nil {
			// Subject already exists, use it
			*subject = existing
			return nil // No change needed, skip revision bump
		}
		if err != gorm.ErrRecordNotFound {
			// Some other error occurred
			return err
		}

		// Subject doesn't exist, create it
		if err := tx.Create(subject).Error; err != nil {
			return err
		}

		// Record change
		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"subject_id":  subject.ID,
			"external_id": subject.ExternalID,
		}
		return AppendChange(tx, tenantID, rev, "SUBJECT_CREATE", OpAdd, payload)
	})
}

// CreateSubjectSet creates a subject attribute (subject set) and records the change
// It is idempotent: if a subject attribute with the same tenant_id and name exists, it returns the existing one
func (h *Handler) CreateSubjectSet(ctx context.Context, tenantID string, ua *UserAttribute) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		ua.TenantID = tenantID

		// Check if subject attribute already exists (idempotent)
		var existing UserAttribute
		err := tx.Where("tenant_id = ? AND name = ?", tenantID, ua.Name).First(&existing).Error
		if err == nil {
			// Subject attribute already exists, use it
			*ua = existing
			return nil // No change needed, skip revision bump
		}
		if err != gorm.ErrRecordNotFound {
			// Some other error occurred
			return err
		}

		// Subject attribute doesn't exist, create it
		if err := tx.Create(ua).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": ua.ID,
			"name":  ua.Name,
		}
		return AppendChange(tx, tenantID, rev, "UA_CREATE", OpAdd, payload)
	})
}

// CreateObject creates an object and records the change
// It is idempotent: if an object with the same tenant_id and external_id exists, it returns the existing object
func (h *Handler) CreateObject(ctx context.Context, tenantID string, obj *Object) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		obj.TenantID = tenantID

		// Check if object already exists (idempotent)
		var existing Object
		err := tx.Where("tenant_id = ? AND external_id = ?", tenantID, obj.ExternalID).First(&existing).Error
		if err == nil {
			// Object already exists, use it
			*obj = existing
			return nil // No change needed, skip revision bump
		}
		if err != gorm.ErrRecordNotFound {
			// Some other error occurred
			return err
		}

		// Object doesn't exist, create it
		if err := tx.Create(obj).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"object_id":   obj.ID,
			"external_id": obj.ExternalID,
		}
		return AppendChange(tx, tenantID, rev, "OBJECT_CREATE", OpAdd, payload)
	})
}

// CreateObjectSet creates an object attribute (object set) and records the change
// It is idempotent: if an object attribute with the same tenant_id and name exists, it returns the existing one
func (h *Handler) CreateObjectSet(ctx context.Context, tenantID string, oa *ObjectAttribute) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		oa.TenantID = tenantID

		// Check if object attribute already exists (idempotent)
		var existing ObjectAttribute
		err := tx.Where("tenant_id = ? AND name = ?", tenantID, oa.Name).First(&existing).Error
		if err == nil {
			// Object attribute already exists, use it
			*oa = existing
			return nil // No change needed, skip revision bump
		}
		if err != gorm.ErrRecordNotFound {
			// Some other error occurred
			return err
		}

		// Object attribute doesn't exist, create it
		if err := tx.Create(oa).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"oa_id": oa.ID,
			"name":  oa.Name,
		}
		return AppendChange(tx, tenantID, rev, "OA_CREATE", OpAdd, payload)
	})
}

// CreateRelationship creates an assignment edge (relationship) and records the change
func (h *Handler) CreateRelationship(ctx context.Context, tenantID string, edge *AssignmentEdge) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		edge.TenantID = tenantID

		// Validate
		if err := validate.ValidateAssignmentEdgeCreate(ctx, tx, tenantID, edge); err != nil {
			return err
		}

		// Check if exists
		var count int64
		err := tx.WithContext(ctx).
			Table("assignment_edges").
			Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
				tenantID, edge.ChildType, edge.ChildID, edge.ParentType, edge.ParentID).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			// Already exists - idempotent
			return nil
		}

		// Create using raw SQL with PostgreSQL placeholders to avoid GORM index issues
		insertSQL := `INSERT INTO assignment_edges (id, tenant_id, child_type, child_id, parent_type, parent_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if err := tx.WithContext(ctx).Exec(insertSQL,
			edge.ID, edge.TenantID, string(edge.ChildType), edge.ChildID, string(edge.ParentType), edge.ParentID, edge.CreatedAt).Error; err != nil {
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
func (h *Handler) DeleteRelationship(ctx context.Context, tenantID string, edge *AssignmentEdge) (int64, error) {
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
func (h *Handler) CreateRule(ctx context.Context, tenantID string, uaID, oaID uuid.UUID, ops []string) (uuid.UUID, int64, error) {
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
			"ops":   ops,
		}
		return AppendChange(tx, tenantID, rev, "ASSOC_OP", OpAdd, payload)
	})
	return assocID, revision, err
}

// DeleteRule deletes an association and records the change
func (h *Handler) DeleteRule(ctx context.Context, tenantID string, assocID uuid.UUID) (int64, error) {
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
			"ops":   operations,
		}
		return AppendChange(tx, tenantID, rev, "ASSOC_OP", OpRemove, payload)
	})
}

// CreateDeny creates a prohibition with operations and records the change
func (h *Handler) CreateDeny(ctx context.Context, tenantID string, subjectType ProhibitionSubjectType, subjectID uuid.UUID, oaID uuid.UUID, ops []string) (uuid.UUID, int64, error) {
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

// Backward compatibility aliases
func (h *Handler) CreateSubjectGroup(ctx context.Context, tenantID string, ua *UserAttribute) (int64, error) {
	return h.CreateSubjectSet(ctx, tenantID, ua)
}

func (h *Handler) CreateObjectGroup(ctx context.Context, tenantID string, oa *ObjectAttribute) (int64, error) {
	return h.CreateObjectSet(ctx, tenantID, oa)
}

// DeleteDeny deletes a prohibition and records the change
func (h *Handler) DeleteDeny(ctx context.Context, tenantID string, prohID uuid.UUID) (int64, error) {
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
func (h *Handler) GetCurrentRevision(ctx context.Context, tenantID string) (int64, error) {
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
func (h *Handler) GetPolicyChanges(ctx context.Context, tenantID string, afterSeq int64, limit int) ([]PolicyChange, error) {
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
