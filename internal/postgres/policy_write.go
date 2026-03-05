package postgres

import (
	"context"
	"fmt"
	"strings"

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

// CreateSubject creates or updates a subject and records the change.
// It is idempotent on (tenant_id, external_id):
// - If the subject does not exist, it is created.
// - If it exists, its mutable fields (kind, tags/metadata) are updated to the latest values.
// Attributes should be created separately via CreateSubjectSet and assigned via CreateRelationship
func (h *Handler) CreateSubject(ctx context.Context, tenantID string, subject *Subject) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		subject.TenantID = tenantID

		// Check if subject already exists (idempotent)
		var existing Subject
		err := tx.Where("tenant_id = ? AND external_id = ?", tenantID, subject.ExternalID).First(&existing).Error
		if err == nil {
			// Subject already exists - update mutable fields to reflect the latest state.
			needsUpdate := false

			if subject.Kind != "" && subject.Kind != existing.Kind {
				existing.Kind = subject.Kind
				needsUpdate = true
			}
			// Always replace tags/metadata if provided (latest snapshot semantics).
			if len(subject.Tags) > 0 {
				existing.Tags = subject.Tags
				needsUpdate = true
			}

			if !needsUpdate {
				// Nothing to change, just return existing.
				*subject = existing
				return nil
			}

			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			*subject = existing

			// Record update change
			rev, _ := BumpRevision(tx, tenantID)
			payload := map[string]any{
				"subject_id":  subject.ID,
				"external_id": subject.ExternalID,
			}
			return AppendChange(tx, tenantID, rev, "SUBJECT_UPDATE", OpUpdate, payload)
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

// CreateSubjectSet creates a subject attribute with attribute_type="custom" and records the change.
// "Subject set" is API-level terminology - this creates a SubjectAttribute with attribute_type="custom".
// By default, creates custom attributes unless AttributeType is explicitly set (but API should enforce custom).
// It is idempotent: if a subject attribute with the same tenant_id and name exists, it returns the existing one.
func (h *Handler) CreateSubjectSet(ctx context.Context, tenantID string, ua *SubjectAttribute) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		ua.TenantID = tenantID

		// Set default attribute type to custom if not specified
		if ua.AttributeType == "" {
			ua.AttributeType = AttributeTypeCustom
		}

		// Check if subject attribute already exists (idempotent)
		var existing SubjectAttribute
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

// CreateSubjectAttributeWithParent creates a subject attribute and optionally creates hierarchy edge if parent is specified
// It is idempotent: if an attribute with the same tenant_id and name exists, it returns the existing one
func (h *Handler) CreateSubjectAttributeWithParent(ctx context.Context, tenantID string, ua *SubjectAttribute, parentName string, parentID *uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		ua.TenantID = tenantID

		// Set default attribute type to native if not specified
		if ua.AttributeType == "" {
			ua.AttributeType = AttributeTypeNative
		}

		// Check if subject attribute already exists (idempotent)
		var existing SubjectAttribute
		err := tx.Where("tenant_id = ? AND name = ?", tenantID, ua.Name).First(&existing).Error
		if err == nil {
			// Subject attribute already exists, use it
			*ua = existing
		} else if err == gorm.ErrRecordNotFound {
			// Subject attribute doesn't exist, create it
			if err := tx.Create(ua).Error; err != nil {
				return err
			}
		} else {
			// Some other error occurred
			return err
		}

		// Handle parent hierarchy if specified
		if parentName != "" || parentID != nil {
			var parentAttrID uuid.UUID
			if parentID != nil {
				parentAttrID = *parentID
			} else {
				// Look up parent by name
				var parentAttr SubjectAttribute
				if err := tx.Where("tenant_id = ? AND name = ?", tenantID, parentName).First(&parentAttr).Error; err != nil {
					return fmt.Errorf("parent attribute %s not found: %w", parentName, err)
				}
				parentAttrID = parentAttr.ID
			}

			// Create hierarchy edge (UA -> UA): child -> parent
			edge := AssignmentEdge{
				TenantID:   tenantID,
				ChildType:  NodeUA,
				ChildID:    ua.ID,
				ParentType: NodeUA,
				ParentID:   parentAttrID,
			}
			// Check if edge exists
			var existingEdge AssignmentEdge
			err := tx.Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
				tenantID, NodeUA, ua.ID, NodeUA, parentAttrID).First(&existingEdge).Error
			if err == gorm.ErrRecordNotFound {
				// Create edge
				if err := tx.Create(&edge).Error; err != nil {
					return fmt.Errorf("failed to create hierarchy edge: %w", err)
				}
			} else if err != nil {
				return err
			}
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": ua.ID,
			"name":  ua.Name,
		}
		return AppendChange(tx, tenantID, rev, "UA_CREATE", OpAdd, payload)
	})
}

// UpdateSubjectAttribute updates a subject attribute
func (h *Handler) UpdateSubjectAttribute(ctx context.Context, tenantID string, uaID uuid.UUID, updates map[string]interface{}) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&SubjectAttribute{}).
			Where("tenant_id = ? AND id = ?", tenantID, uaID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id":   uaID,
			"updates": updates,
		}
		return AppendChange(tx, tenantID, rev, "UA_UPDATE", OpUpdate, payload)
	})
}

// DeleteSubjectAttribute deletes a subject attribute (alias for DeleteSubjectSet)
func (h *Handler) DeleteSubjectAttribute(ctx context.Context, tenantID string, uaID uuid.UUID) (int64, error) {
	return h.DeleteSubjectSet(ctx, tenantID, uaID)
}

// CreateObject creates or updates an object and records the change.
// It is idempotent on (tenant_id, external_id):
// - If the object does not exist, it is created.
// - If it exists, its mutable fields (kind, attribute_type, display_name, tags/metadata) are updated.
// Attributes should be created separately via CreateObjectSet and assigned via CreateRelationship
func (h *Handler) CreateObject(ctx context.Context, tenantID string, obj *Object) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		obj.TenantID = tenantID

		// Check if object already exists (idempotent)
		var existing Object
		err := tx.Where("tenant_id = ? AND external_id = ?", tenantID, obj.ExternalID).First(&existing).Error
		if err == nil {
			// Object already exists - update mutable fields to reflect latest state.
			needsUpdate := false

			if obj.Kind != "" && obj.Kind != existing.Kind {
				existing.Kind = obj.Kind
				needsUpdate = true
			}
			if obj.AttributeType != "" && obj.AttributeType != existing.AttributeType {
				existing.AttributeType = obj.AttributeType
				needsUpdate = true
			}
			if obj.DisplayName != "" && obj.DisplayName != existing.DisplayName {
				existing.DisplayName = obj.DisplayName
				needsUpdate = true
			}
			// Always replace tags/metadata if provided (latest snapshot semantics).
			if len(obj.Tags) > 0 {
				existing.Tags = obj.Tags
				needsUpdate = true
			}

			if !needsUpdate {
				*obj = existing
				return nil
			}

			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			*obj = existing

			rev, _ := BumpRevision(tx, tenantID)
			payload := map[string]any{
				"object_id":   obj.ID,
				"external_id": obj.ExternalID,
			}
			return AppendChange(tx, tenantID, rev, "OBJECT_UPDATE", OpUpdate, payload)
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

// CreateObjectSet creates an object attribute with attribute_type="custom" and records the change.
// "Object set" is API-level terminology - this creates an ObjectAttribute with attribute_type="custom".
// By default, creates custom attributes unless AttributeType is explicitly set (but API should enforce custom).
// It is idempotent: if an object attribute with the same tenant_id and name exists, it returns the existing one.
func (h *Handler) CreateObjectSet(ctx context.Context, tenantID string, oa *ObjectAttribute) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		oa.TenantID = tenantID

		// Set default attribute type to custom if not specified
		if oa.AttributeType == "" {
			oa.AttributeType = AttributeTypeCustom
		}

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

// CreateAssociationForAttributes creates or updates an association between a subject attribute (UA)
// and an object attribute (OA) with the given operations, and bumps the policy revision.
// This is a small wrapper around CreateAssociation for RBAC-style role permissions.
func (h *Handler) CreateAssociationForAttributes(ctx context.Context, tenantID string, uaID, oaID uuid.UUID, ops []string) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		if err := CreateAssociation(ctx, tx, tenantID, uaID, oaID, ops); err != nil {
			return err
		}
		return nil
	})
}

// CreateObjectAttributeWithParent creates an object attribute and optionally creates hierarchy edge if parent is specified
// It is idempotent: if an attribute with the same tenant_id and name exists, it returns the existing one
func (h *Handler) CreateObjectAttributeWithParent(ctx context.Context, tenantID string, oa *ObjectAttribute, parentName string, parentID *uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		oa.TenantID = tenantID

		// Set default attribute type to native if not specified
		if oa.AttributeType == "" {
			oa.AttributeType = AttributeTypeNative
		}

		// Check if object attribute already exists (idempotent)
		var existing ObjectAttribute
		err := tx.Where("tenant_id = ? AND name = ?", tenantID, oa.Name).First(&existing).Error
		if err == nil {
			// Object attribute already exists, use it
			*oa = existing
		} else if err == gorm.ErrRecordNotFound {
			// Object attribute doesn't exist, create it
			if err := tx.Create(oa).Error; err != nil {
				return err
			}
		} else {
			// Some other error occurred
			return err
		}

		// Handle parent hierarchy if specified
		if parentName != "" || parentID != nil {
			var parentAttrID uuid.UUID
			if parentID != nil {
				parentAttrID = *parentID
			} else {
				// Look up parent by name
				var parentAttr ObjectAttribute
				if err := tx.Where("tenant_id = ? AND name = ?", tenantID, parentName).First(&parentAttr).Error; err != nil {
					return fmt.Errorf("parent attribute %s not found: %w", parentName, err)
				}
				parentAttrID = parentAttr.ID
			}

			// Create hierarchy edge (OA -> OA): child -> parent
			edge := AssignmentEdge{
				TenantID:   tenantID,
				ChildType:  NodeOA,
				ChildID:    oa.ID,
				ParentType: NodeOA,
				ParentID:   parentAttrID,
			}
			// Check if edge exists
			var existingEdge AssignmentEdge
			err := tx.Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
				tenantID, NodeOA, oa.ID, NodeOA, parentAttrID).First(&existingEdge).Error
			if err == gorm.ErrRecordNotFound {
				// Create edge
				if err := tx.Create(&edge).Error; err != nil {
					return fmt.Errorf("failed to create hierarchy edge: %w", err)
				}
			} else if err != nil {
				return err
			}
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"oa_id": oa.ID,
			"name":  oa.Name,
		}
		return AppendChange(tx, tenantID, rev, "OA_CREATE", OpAdd, payload)
	})
}

// UpdateObjectAttribute updates an object attribute
func (h *Handler) UpdateObjectAttribute(ctx context.Context, tenantID string, oaID uuid.UUID, updates map[string]interface{}) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&ObjectAttribute{}).
			Where("tenant_id = ? AND id = ?", tenantID, oaID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"oa_id":   oaID,
			"updates": updates,
		}
		return AppendChange(tx, tenantID, rev, "OA_UPDATE", OpUpdate, payload)
	})
}

// DeleteObjectAttribute deletes an object attribute (alias for DeleteObjectSet)
func (h *Handler) DeleteObjectAttribute(ctx context.Context, tenantID string, oaID uuid.UUID) (int64, error) {
	return h.DeleteObjectSet(ctx, tenantID, oaID)
}

// CreateRelationship creates an assignment edge (relationship) and records the change
func (h *Handler) CreateRelationship(ctx context.Context, tenantID string, edge *AssignmentEdge) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		edge.TenantID = tenantID

		// Validate - this already checks for duplicates and returns nil if exists (idempotent)
		if err := validate.ValidateAssignmentEdgeCreate(ctx, tx, tenantID, edge); err != nil {
			return err
		}

		// Check if edge already exists (validation returns nil if exists, but we need to check explicitly)
		var existing AssignmentEdge
		err := tx.WithContext(ctx).
			Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
			tenantID, edge.ChildType, edge.ChildID, edge.ParentType, edge.ParentID).
			First(&existing).Error
		if err == nil {
			return nil // Already exists - idempotent
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		// Create the edge
		if err := tx.Create(edge).Error; err != nil {
			// Handle duplicate key errors (idempotent)
			errMsg := err.Error()
			if strings.Contains(errMsg, "23505") || strings.Contains(errMsg, "duplicate key") ||
				strings.Contains(errMsg, "unique constraint") || strings.Contains(errMsg, "uidx_asg_edge") {
				return nil // Already exists - idempotent
			}
			return err
		}

		// Record change
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
// subjectType: "subject" or "subject-attribute" (UA)
// objectType: "object" or "object-attribute" (OA)
// Note: "subject-set" and "object-set" are API terminology only for the subject-sets/object-sets endpoints.
// In rules API, we use "subject-attribute" and "object-attribute" to refer to the actual entities.
func (h *Handler) CreateRule(ctx context.Context, tenantID string, subjectType string, subjectID uuid.UUID, objectType string, objectID uuid.UUID, ops []string) (uuid.UUID, int64, error) {
	var assocID uuid.UUID
	revision, err := h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		assoc := Association{
			TenantID: tenantID,
		}

		// Set subject side
		if subjectType == "subject" {
			assoc.SubjectID = &subjectID
		} else if subjectType == "subject-attribute" {
			assoc.SubjectAttributeID = &subjectID
		} else {
			return fmt.Errorf("invalid subject_type: %s (must be 'subject' or 'subject-attribute')", subjectType)
		}

		// Set object side
		if objectType == "object" {
			assoc.ObjectID = &objectID
		} else if objectType == "object-attribute" {
			assoc.ObjectAttributeID = &objectID
		} else {
			return fmt.Errorf("invalid object_type: %s (must be 'object' or 'object-attribute')", objectType)
		}

		// Build unique constraint query
		query := tx.Where("tenant_id = ?", tenantID)
		if assoc.SubjectID != nil {
			query = query.Where("subject_id = ?", *assoc.SubjectID).Where("subject_attribute_id IS NULL")
		} else {
			query = query.Where("subject_attribute_id = ?", *assoc.SubjectAttributeID).Where("subject_id IS NULL")
		}
		if assoc.ObjectID != nil {
			query = query.Where("object_id = ?", *assoc.ObjectID).Where("object_attribute_id IS NULL")
		} else {
			query = query.Where("object_attribute_id = ?", *assoc.ObjectAttributeID).Where("object_id IS NULL")
		}

		if err := query.FirstOrCreate(&assoc).Error; err != nil {
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
			"subject_type": subjectType,
			"subject_id":   subjectID,
			"object_type":  objectType,
			"object_id":    objectID,
			"ops":          ops,
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
			"ops": operations,
		}
		if assoc.SubjectID != nil {
			payload["subject_type"] = "subject"
			payload["subject_id"] = *assoc.SubjectID
		} else if assoc.SubjectAttributeID != nil {
			payload["subject_type"] = "subject-attribute"
			payload["subject_id"] = *assoc.SubjectAttributeID
		}
		if assoc.ObjectID != nil {
			payload["object_type"] = "object"
			payload["object_id"] = *assoc.ObjectID
		} else if assoc.ObjectAttributeID != nil {
			payload["object_type"] = "object-attribute"
			payload["object_id"] = *assoc.ObjectAttributeID
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
