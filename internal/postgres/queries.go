package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListSubjects returns paginated list of subjects with optional attribute filtering
// attributeFilters: list of attribute IDs to filter by (subjects must be assigned to at least one of these)
func (h *Handler) ListSubjects(ctx context.Context, tenantID string, query string, limit int, cursor string, attributeFilters []string) ([]Subject, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Model(&Subject{}).Where("subjects.tenant_id = ?", tenantID)

	// Apply search query
	if query != "" {
		search := "%" + query + "%"
		db = db.Where("subjects.external_id ILIKE ? OR subjects.email ILIKE ? OR subjects.display ILIKE ?", search, search, search)
	}

	// Apply attribute filters - subjects must be assigned to at least one of the specified attributes
	if len(attributeFilters) > 0 {
		var attributeIDs []uuid.UUID
		for _, attrIDStr := range attributeFilters {
			if attrID, err := uuid.Parse(attrIDStr); err == nil {
				attributeIDs = append(attributeIDs, attrID)
			}
		}
		if len(attributeIDs) > 0 {
			db = db.Joins("INNER JOIN assignment_edges ON subjects.id = assignment_edges.child_id").
				Where("assignment_edges.tenant_id = ? AND assignment_edges.child_type = ? AND assignment_edges.parent_type = ? AND assignment_edges.parent_id IN ?",
					tenantID, NodeSubject, NodeUA, attributeIDs).
				Group("subjects.id")
		}
	}

	// Apply cursor (simple: use ID > cursor)
	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("subjects.id > ?", cursorID)
		}
	}

	var subjects []Subject
	err := db.Order("subjects.id ASC").Limit(limit + 1).Find(&subjects).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(subjects) > limit
	if hasMore {
		subjects = subjects[:limit]
	}

	nextCursor := ""
	if len(subjects) > 0 {
		nextCursor = subjects[len(subjects)-1].ID.String()
	}

	return subjects, nextCursor, hasMore, nil
}

// GetSubject returns a subject by ID
func (h *Handler) GetSubject(ctx context.Context, tenantID string, subjectID uuid.UUID) (*Subject, error) {
	var subject Subject
	err := h.H.WithContext(ctx).
		Model(&Subject{}).
		Where("tenant_id = ? AND id = ?", tenantID, subjectID).
		First(&subject).Error
	if err != nil {
		return nil, err
	}
	return &subject, nil
}

// UpdateSubject updates a subject
func (h *Handler) UpdateSubject(ctx context.Context, tenantID string, subjectID uuid.UUID, updates map[string]interface{}) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&Subject{}).
			Where("tenant_id = ? AND id = ?", tenantID, subjectID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"subject_id": subjectID,
			"updates":    updates,
		}
		return AppendChange(tx, tenantID, rev, "SUBJECT_UPDATE", OpUpdate, payload)
	})
}

// DeleteSubject deletes a subject
func (h *Handler) DeleteSubject(ctx context.Context, tenantID string, subjectID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var subject Subject
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, subjectID).First(&subject).Error; err != nil {
			return err
		}

		if err := tx.Delete(&subject).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"subject_id": subjectID,
		}
		return AppendChange(tx, tenantID, rev, "SUBJECT_DELETE", OpRemove, payload)
	})
}

// ListSubjectSets returns paginated list of subject attributes with attribute_type="custom".
// "Subject set" is API terminology - this returns SubjectAttributes where attribute_type="custom".
// Native/system attributes (attribute_type="native") are excluded from subject sets.
func (h *Handler) ListSubjectSets(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]SubjectAttribute, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	// Only return custom attributes (subject sets should not include native/system attributes)
	db := h.H.WithContext(ctx).Model(&SubjectAttribute{}).
		Where("tenant_id = ? AND attribute_type = ?", tenantID, AttributeTypeCustom)

	if query != "" {
		search := "%" + query + "%"
		db = db.Where("name ILIKE ?", search)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var uas []SubjectAttribute
	err := db.Order("id ASC").Limit(limit + 1).Find(&uas).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(uas) > limit
	if hasMore {
		uas = uas[:limit]
	}

	nextCursor := ""
	if len(uas) > 0 {
		nextCursor = uas[len(uas)-1].ID.String()
	}

	return uas, nextCursor, hasMore, nil
}

// GetSubjectSet returns a subject attribute by ID.
// Note: This can return any attribute (custom or native) - the API endpoint should validate attribute_type="custom" if needed.
func (h *Handler) GetSubjectSet(ctx context.Context, tenantID string, uaID uuid.UUID) (*SubjectAttribute, error) {
	var ua SubjectAttribute
	err := h.H.WithContext(ctx).
		Model(&SubjectAttribute{}).
		Where("tenant_id = ? AND id = ?", tenantID, uaID).
		First(&ua).Error
	if err != nil {
		return nil, err
	}
	return &ua, nil
}

// GetSubjectSetMembers returns all subject IDs that are members of a subject attribute.
// Works for both custom and native attributes.
func (h *Handler) GetSubjectSetMembers(ctx context.Context, tenantID string, uaID uuid.UUID) ([]uuid.UUID, error) {
	var edges []AssignmentEdge
	err := h.H.WithContext(ctx).
		Model(&AssignmentEdge{}).
		Where("tenant_id = ? AND parent_type = ? AND parent_id = ? AND child_type = ?", tenantID, NodeUA, uaID, NodeSubject).
		Find(&edges).Error
	if err != nil {
		return nil, err
	}

	memberIDs := make([]uuid.UUID, len(edges))
	for i, edge := range edges {
		memberIDs[i] = edge.ChildID
	}
	return memberIDs, nil
}

// GetSubjectAttributes returns all attributes (both native and custom) assigned to a subject
// This queries assignment edges where the subject is the child and UA (subject attribute) is the parent
func (h *Handler) GetSubjectAttributes(ctx context.Context, tenantID string, subjectID uuid.UUID) ([]SubjectAttribute, error) {
	var attributes []SubjectAttribute
	err := h.H.WithContext(ctx).
		Table("subject_attributes").
		Joins("INNER JOIN assignment_edges ON subject_attributes.id = assignment_edges.parent_id").
		Where("assignment_edges.tenant_id = ? AND assignment_edges.child_type = ? AND assignment_edges.child_id = ? AND assignment_edges.parent_type = ?",
			tenantID, NodeSubject, subjectID, NodeUA).
		Find(&attributes).Error
	if err != nil {
		return nil, err
	}
	return attributes, nil
}

// GetSubjectAttributeSubgraph returns the complete attribute tree for a subject,
// where the subject is the root node and attributes are child nodes.
// Returns attributes (nodes) and their hierarchical relationships (edges), including subject->UA edges.
func (h *Handler) GetSubjectAttributeSubgraph(ctx context.Context, tenantID string, subjectID uuid.UUID) ([]SubjectAttribute, []AssignmentEdge, error) {
	// Step 1: Get direct edges (subject -> UA) and include them in the result
	var directEdges []AssignmentEdge
	err := h.H.WithContext(ctx).
		Model(&AssignmentEdge{}).
		Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ?",
			tenantID, NodeSubject, subjectID, NodeUA).
		Find(&directEdges).Error
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Collect all attribute IDs from direct edges
	attributeIDs := make(map[uuid.UUID]bool)
	for _, edge := range directEdges {
		attributeIDs[edge.ParentID] = true
	}

	// Step 3: Recursively find all descendant attributes via UA->UA edges (downward only)
	// We use a queue-based BFS approach, traversing only downward (children)
	queue := make([]uuid.UUID, 0, len(attributeIDs))
	for id := range attributeIDs {
		queue = append(queue, id)
	}

	var allEdges []AssignmentEdge
	allEdges = append(allEdges, directEdges...) // Include subject->UA edges
	visited := make(map[uuid.UUID]bool)

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		// Find all child UA nodes (children in the hierarchy) - downward traversal only
		var childEdges []AssignmentEdge
		err := h.H.WithContext(ctx).
			Model(&AssignmentEdge{}).
			Where("tenant_id = ? AND parent_id = ? AND child_type = ? AND parent_type = ?",
				tenantID, currentID, NodeUA, NodeUA).
			Find(&childEdges).Error
		if err != nil {
			return nil, nil, err
		}

		allEdges = append(allEdges, childEdges...)
		for _, edge := range childEdges {
			if !attributeIDs[edge.ChildID] {
				attributeIDs[edge.ChildID] = true
				queue = append(queue, edge.ChildID)
			}
		}
	}

	// Step 4: Fetch all attribute details
	if len(attributeIDs) == 0 {
		return []SubjectAttribute{}, allEdges, nil
	}

	idsList := make([]uuid.UUID, 0, len(attributeIDs))
	for id := range attributeIDs {
		idsList = append(idsList, id)
	}

	var allAttributes []SubjectAttribute
	err = h.H.WithContext(ctx).
		Model(&SubjectAttribute{}).
		Where("tenant_id = ? AND id IN ?", tenantID, idsList).
		Find(&allAttributes).Error
	if err != nil {
		return nil, nil, err
	}

	return allAttributes, allEdges, nil
}

// ListSubjectAttributes returns all subject attributes (both native and custom)
func (h *Handler) ListSubjectAttributes(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]SubjectAttribute, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	// Return all attributes (both native and custom)
	db := h.H.WithContext(ctx).Model(&SubjectAttribute{}).
		Where("tenant_id = ?", tenantID)

	if query != "" {
		search := "%" + query + "%"
		db = db.Where("name ILIKE ?", search)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var uas []SubjectAttribute
	err := db.Order("id ASC").Limit(limit + 1).Find(&uas).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(uas) > limit
	if hasMore {
		uas = uas[:limit]
	}

	nextCursor := ""
	if len(uas) > 0 {
		nextCursor = uas[len(uas)-1].ID.String()
	}

	return uas, nextCursor, hasMore, nil
}

// GetSubjectAttribute returns a subject attribute by ID (alias for GetSubjectSet)
func (h *Handler) GetSubjectAttribute(ctx context.Context, tenantID string, uaID uuid.UUID) (*SubjectAttribute, error) {
	return h.GetSubjectSet(ctx, tenantID, uaID)
}

// UpdateSubjectSet updates a subject attribute (typically used for custom attributes via subject-sets API).
func (h *Handler) UpdateSubjectSet(ctx context.Context, tenantID string, uaID uuid.UUID, name string) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&SubjectAttribute{}).
			Where("tenant_id = ? AND id = ?", tenantID, uaID).
			Update("name", name)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": uaID,
			"name":  name,
		}
		return AppendChange(tx, tenantID, rev, "UA_UPDATE", OpUpdate, payload)
	})
}

// DeleteSubjectSet deletes a subject attribute (typically used for custom attributes via subject-sets API).
func (h *Handler) DeleteSubjectSet(ctx context.Context, tenantID string, uaID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var ua SubjectAttribute
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, uaID).First(&ua).Error; err != nil {
			return err
		}

		if err := tx.Delete(&ua).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": uaID,
		}
		return AppendChange(tx, tenantID, rev, "UA_DELETE", OpRemove, payload)
	})
}

// ListObjects returns paginated list of objects with optional attribute filtering
// attributeFilters: list of attribute IDs to filter by (objects must be assigned to at least one of these)
func (h *Handler) ListObjects(ctx context.Context, tenantID string, query string, limit int, cursor string, attributeFilters []string) ([]Object, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Model(&Object{}).Where("objects.tenant_id = ?", tenantID)

	if query != "" {
		search := "%" + query + "%"
		db = db.Where("objects.external_id ILIKE ? OR objects.attribute_type ILIKE ? OR objects.display_name ILIKE ?", search, search, search)
	}

	// Apply attribute filters - objects must be assigned to at least one of the specified attributes
	if len(attributeFilters) > 0 {
		var attributeIDs []uuid.UUID
		for _, attrIDStr := range attributeFilters {
			if attrID, err := uuid.Parse(attrIDStr); err == nil {
				attributeIDs = append(attributeIDs, attrID)
			}
		}
		if len(attributeIDs) > 0 {
			db = db.Joins("INNER JOIN assignment_edges ON objects.id = assignment_edges.child_id").
				Where("assignment_edges.tenant_id = ? AND assignment_edges.child_type = ? AND assignment_edges.parent_type = ? AND assignment_edges.parent_id IN ?",
					tenantID, NodeObject, NodeOA, attributeIDs).
				Group("objects.id")
		}
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("objects.id > ?", cursorID)
		}
	}

	var objects []Object
	err := db.Order("objects.id ASC").Limit(limit + 1).Find(&objects).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(objects) > limit
	if hasMore {
		objects = objects[:limit]
	}

	nextCursor := ""
	if len(objects) > 0 {
		nextCursor = objects[len(objects)-1].ID.String()
	}

	return objects, nextCursor, hasMore, nil
}

// GetObject returns an object by ID
func (h *Handler) GetObject(ctx context.Context, tenantID string, objectID uuid.UUID) (*Object, error) {
	var obj Object
	err := h.H.WithContext(ctx).
		Model(&Object{}).
		Where("tenant_id = ? AND id = ?", tenantID, objectID).
		First(&obj).Error
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

// UpdateObject updates an object
func (h *Handler) UpdateObject(ctx context.Context, tenantID string, objectID uuid.UUID, updates map[string]interface{}) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&Object{}).
			Where("tenant_id = ? AND id = ?", tenantID, objectID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"object_id": objectID,
			"updates":   updates,
		}
		return AppendChange(tx, tenantID, rev, "OBJECT_UPDATE", OpUpdate, payload)
	})
}

// DeleteObject deletes an object
func (h *Handler) DeleteObject(ctx context.Context, tenantID string, objectID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var obj Object
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, objectID).First(&obj).Error; err != nil {
			return err
		}

		if err := tx.Delete(&obj).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"object_id": objectID,
		}
		return AppendChange(tx, tenantID, rev, "OBJECT_DELETE", OpRemove, payload)
	})
}

// ListObjectSets returns paginated list of object attributes with attribute_type="custom".
// "Object set" is API terminology - this returns ObjectAttributes where attribute_type="custom".
// Native/system attributes (attribute_type="native") are excluded from object sets.
func (h *Handler) ListObjectSets(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]ObjectAttribute, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	// Only return custom attributes (object sets should not include native/system attributes)
	db := h.H.WithContext(ctx).Model(&ObjectAttribute{}).
		Where("tenant_id = ? AND attribute_type = ?", tenantID, AttributeTypeCustom)

	if query != "" {
		search := "%" + query + "%"
		db = db.Where("name ILIKE ?", search)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var oas []ObjectAttribute
	err := db.Order("id ASC").Limit(limit + 1).Find(&oas).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(oas) > limit
	if hasMore {
		oas = oas[:limit]
	}

	nextCursor := ""
	if len(oas) > 0 {
		nextCursor = oas[len(oas)-1].ID.String()
	}

	return oas, nextCursor, hasMore, nil
}

// GetObjectSet returns an object attribute by ID.
// Note: This can return any attribute (custom or native) - the API endpoint should validate attribute_type="custom" if needed.
func (h *Handler) GetObjectSet(ctx context.Context, tenantID string, oaID uuid.UUID) (*ObjectAttribute, error) {
	var oa ObjectAttribute
	err := h.H.WithContext(ctx).
		Model(&ObjectAttribute{}).
		Where("tenant_id = ? AND id = ?", tenantID, oaID).
		First(&oa).Error
	if err != nil {
		return nil, err
	}
	return &oa, nil
}

// GetObjectSetMembers returns all object IDs that are members of an object attribute.
// Works for both custom and native attributes.
func (h *Handler) GetObjectSetMembers(ctx context.Context, tenantID string, oaID uuid.UUID) ([]uuid.UUID, error) {
	var edges []AssignmentEdge
	err := h.H.WithContext(ctx).
		Model(&AssignmentEdge{}).
		Where("tenant_id = ? AND parent_type = ? AND parent_id = ? AND child_type = ?", tenantID, NodeOA, oaID, NodeObject).
		Find(&edges).Error
	if err != nil {
		return nil, err
	}

	memberIDs := make([]uuid.UUID, len(edges))
	for i, edge := range edges {
		memberIDs[i] = edge.ChildID
	}
	return memberIDs, nil
}

// GetObjectAttributes returns all attributes (both native and custom) assigned to an object
// This is used for the object detail page to show all attributes
func (h *Handler) GetObjectAttributes(ctx context.Context, tenantID string, objectID uuid.UUID) ([]ObjectAttribute, error) {
	var attributes []ObjectAttribute
	err := h.H.WithContext(ctx).
		Table("object_attributes").
		Joins("INNER JOIN assignment_edges ON object_attributes.id = assignment_edges.parent_id").
		Where("assignment_edges.tenant_id = ? AND assignment_edges.child_type = ? AND assignment_edges.child_id = ? AND assignment_edges.parent_type = ?",
			tenantID, NodeObject, objectID, NodeOA).
		Find(&attributes).Error
	if err != nil {
		return nil, err
	}
	return attributes, nil
}

// GetObjectAttributeSubgraph returns the complete attribute tree for an object,
// where the object is the root node and attributes are child nodes.
// Returns attributes (nodes) and their hierarchical relationships (edges), including object->OA edges.
func (h *Handler) GetObjectAttributeSubgraph(ctx context.Context, tenantID string, objectID uuid.UUID) ([]ObjectAttribute, []AssignmentEdge, error) {
	// Step 1: Get direct edges (object -> OA) and include them in the result
	var directEdges []AssignmentEdge
	err := h.H.WithContext(ctx).
		Model(&AssignmentEdge{}).
		Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ?",
			tenantID, NodeObject, objectID, NodeOA).
		Find(&directEdges).Error
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Collect all attribute IDs from direct edges
	attributeIDs := make(map[uuid.UUID]bool)
	for _, edge := range directEdges {
		attributeIDs[edge.ParentID] = true
	}

	// Step 3: Recursively find all descendant attributes via OA->OA edges (downward only)
	// We use a queue-based BFS approach, traversing only downward (children)
	queue := make([]uuid.UUID, 0, len(attributeIDs))
	for id := range attributeIDs {
		queue = append(queue, id)
	}

	var allEdges []AssignmentEdge
	allEdges = append(allEdges, directEdges...) // Include object->OA edges
	visited := make(map[uuid.UUID]bool)

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		// Find all child OA nodes (children in the hierarchy) - downward traversal only
		var childEdges []AssignmentEdge
		err := h.H.WithContext(ctx).
			Model(&AssignmentEdge{}).
			Where("tenant_id = ? AND parent_id = ? AND child_type = ? AND parent_type = ?",
				tenantID, currentID, NodeOA, NodeOA).
			Find(&childEdges).Error
		if err != nil {
			return nil, nil, err
		}

		allEdges = append(allEdges, childEdges...)
		for _, edge := range childEdges {
			if !attributeIDs[edge.ChildID] {
				attributeIDs[edge.ChildID] = true
				queue = append(queue, edge.ChildID)
			}
		}
	}

	// Step 4: Fetch all attribute details
	if len(attributeIDs) == 0 {
		return []ObjectAttribute{}, allEdges, nil
	}

	idsList := make([]uuid.UUID, 0, len(attributeIDs))
	for id := range attributeIDs {
		idsList = append(idsList, id)
	}

	var allAttributes []ObjectAttribute
	err = h.H.WithContext(ctx).
		Model(&ObjectAttribute{}).
		Where("tenant_id = ? AND id IN ?", tenantID, idsList).
		Find(&allAttributes).Error
	if err != nil {
		return nil, nil, err
	}

	return allAttributes, allEdges, nil
}

// GetAvailableObjectAttributes returns all available attributes (both native and custom) for filtering
// This is used to populate filter options on the object list page
func (h *Handler) GetAvailableObjectAttributes(ctx context.Context, tenantID string) ([]ObjectAttribute, error) {
	var attributes []ObjectAttribute
	err := h.H.WithContext(ctx).
		Model(&ObjectAttribute{}).
		Where("tenant_id = ?", tenantID).
		Order("attribute_type ASC, name ASC").
		Find(&attributes).Error
	if err != nil {
		return nil, err
	}
	return attributes, nil
}

// UpdateObjectSet updates an object attribute (typically used for custom attributes via object-sets API).
func (h *Handler) UpdateObjectSet(ctx context.Context, tenantID string, oaID uuid.UUID, name string) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&ObjectAttribute{}).
			Where("tenant_id = ? AND id = ?", tenantID, oaID).
			Update("name", name)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"oa_id": oaID,
			"name":  name,
		}
		return AppendChange(tx, tenantID, rev, "OA_UPDATE", OpUpdate, payload)
	})
}

// ListObjectAttributes returns all object attributes (both native and custom)
func (h *Handler) ListObjectAttributes(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]ObjectAttribute, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	// Return all attributes (both native and custom)
	db := h.H.WithContext(ctx).Model(&ObjectAttribute{}).
		Where("tenant_id = ?", tenantID)

	if query != "" {
		search := "%" + query + "%"
		db = db.Where("name ILIKE ?", search)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var oas []ObjectAttribute
	err := db.Order("id ASC").Limit(limit + 1).Find(&oas).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(oas) > limit
	if hasMore {
		oas = oas[:limit]
	}

	nextCursor := ""
	if len(oas) > 0 {
		nextCursor = oas[len(oas)-1].ID.String()
	}

	return oas, nextCursor, hasMore, nil
}

// GetObjectAttribute returns an object attribute by ID (alias for GetObjectSet)
func (h *Handler) GetObjectAttribute(ctx context.Context, tenantID string, oaID uuid.UUID) (*ObjectAttribute, error) {
	return h.GetObjectSet(ctx, tenantID, oaID)
}

// DeleteObjectSet deletes an object attribute (typically used for custom attributes via object-sets API).
func (h *Handler) DeleteObjectSet(ctx context.Context, tenantID string, oaID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var oa ObjectAttribute
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, oaID).First(&oa).Error; err != nil {
			return err
		}

		if err := tx.Delete(&oa).Error; err != nil {
			return err
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"oa_id": oaID,
		}
		return AppendChange(tx, tenantID, rev, "OA_DELETE", OpRemove, payload)
	})
}

// MapRelationshipKindToEdgeTypes converts relationship API type names to NGAC edge types.
func MapRelationshipKindToEdgeTypes(kind string, fromType, toType string) (childType, parentType NodeType, err error) {
	switch kind {
	case "subject_member_of_set", "subject_member_of_group", "subject_member_of_attribute":
		if fromType != "subject" || (toType != "subject-attribute" && toType != "subject-group") {
			return "", "", fmt.Errorf("invalid types for subject_member_of_set: from must be 'subject', to must be 'subject-attribute' or 'subject-group'")
		}
		return NodeSubject, NodeUA, nil
	case "subject_set_parent_of_set", "subject_group_parent_of_group", "subject_attribute_parent_of_attribute":
		if (fromType != "subject-attribute" && fromType != "subject-group") || (toType != "subject-attribute" && toType != "subject-group") {
			return "", "", fmt.Errorf("invalid types for subject_set_parent_of_set: both from and to must be 'subject-attribute' or 'subject-group'")
		}
		return NodeUA, NodeUA, nil
	case "object_member_of_set", "object_member_of_group", "object_member_of_attribute":
		if fromType != "object" || (toType != "object-attribute" && toType != "object-group") {
			return "", "", fmt.Errorf("invalid types for object_member_of_set: from must be 'object', to must be 'object-attribute' or 'object-group'")
		}
		return NodeObject, NodeOA, nil
	case "object_set_parent_of_set", "object_group_parent_of_group", "object_attribute_parent_of_attribute":
		if (fromType != "object-attribute" && fromType != "object-group") || (toType != "object-attribute" && toType != "object-group") {
			return "", "", fmt.Errorf("invalid types for object_set_parent_of_set: both from and to must be 'object-attribute' or 'object-group'")
		}
		return NodeOA, NodeOA, nil
	default:
		return "", "", fmt.Errorf("unknown relationship kind: %s", kind)
	}
}

// ListRelationships returns paginated list of assignment edges
func (h *Handler) ListRelationships(ctx context.Context, tenantID string, filters map[string]interface{}, limit int, cursor string) ([]AssignmentEdge, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Model(&AssignmentEdge{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if kind, ok := filters["kind"].(string); ok && kind != "" {
		// Filter by child/parent type based on kind
		switch kind {
		case "subject_member_of_set", "subject_member_of_group", "subject_member_of_attribute":
			db = db.Where("child_type = ? AND parent_type = ?", NodeSubject, NodeUA)
		case "subject_set_parent_of_set", "subject_group_parent_of_group", "subject_attribute_parent_of_attribute":
			db = db.Where("child_type = ? AND parent_type = ?", NodeUA, NodeUA)
		case "object_member_of_set", "object_member_of_group", "object_member_of_attribute":
			db = db.Where("child_type = ? AND parent_type = ?", NodeObject, NodeOA)
		case "object_set_parent_of_set", "object_group_parent_of_group", "object_attribute_parent_of_attribute":
			db = db.Where("child_type = ? AND parent_type = ?", NodeOA, NodeOA)
		}
	}

	if fromType, ok := filters["from_type"].(string); ok && fromType != "" {
		// Map relationship API type to NodeType
		var nodeType NodeType
		switch fromType {
		case "subject":
			nodeType = NodeSubject
		case "subject-attribute", "subject-group":
			nodeType = NodeUA
		case "object":
			nodeType = NodeObject
		case "object-attribute", "object-group":
			nodeType = NodeOA
		default:
			nodeType = NodeType(fromType)
		}
		db = db.Where("child_type = ?", nodeType)
	}

	if fromID, ok := filters["from_id"].(uuid.UUID); ok && fromID != uuid.Nil {
		db = db.Where("child_id = ?", fromID)
	}

	if toType, ok := filters["to_type"].(string); ok && toType != "" {
		// Map relationship API type to NodeType
		var nodeType NodeType
		switch toType {
		case "subject-attribute", "subject-group":
			nodeType = NodeUA
		case "object-attribute", "object-group":
			nodeType = NodeOA
		default:
			nodeType = NodeType(toType)
		}
		db = db.Where("parent_type = ?", nodeType)
	}

	if toID, ok := filters["to_id"].(uuid.UUID); ok && toID != uuid.Nil {
		db = db.Where("parent_id = ?", toID)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var edges []AssignmentEdge
	err := db.Order("id ASC").Limit(limit + 1).Find(&edges).Error
	if err != nil {
		return nil, "", false, err
	}

	hasMore := len(edges) > limit
	if hasMore {
		edges = edges[:limit]
	}

	nextCursor := ""
	if len(edges) > 0 {
		nextCursor = edges[len(edges)-1].ID.String()
	}

	return edges, nextCursor, hasMore, nil
}

// ListRules returns paginated list of associations with operations
func (h *Handler) ListRules(ctx context.Context, tenantID string, filters map[string]interface{}, limit int, cursor string) ([]Association, []map[string]interface{}, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Model(&Association{}).Where("tenant_id = ?", tenantID)

	// Apply filters - support both subject/subject-set and object/object-set
	if subjectID, ok := filters["subject_scope_id"].(uuid.UUID); ok && subjectID != uuid.Nil {
		// Filter by either subject_id or subject_attribute_id
		db = db.Where("(subject_id = ? OR subject_attribute_id = ?)", subjectID, subjectID)
	}

	if objectID, ok := filters["object_scope_id"].(uuid.UUID); ok && objectID != uuid.Nil {
		// Filter by either object_id or object_attribute_id
		db = db.Where("(object_id = ? OR object_attribute_id = ?)", objectID, objectID)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var assocs []Association
	err := db.Order("id ASC").Limit(limit + 1).Find(&assocs).Error
	if err != nil {
		return nil, nil, "", false, err
	}

	hasMore := len(assocs) > limit
	if hasMore {
		assocs = assocs[:limit]
	}

	// Load operations for each association
	opsMap := make(map[uuid.UUID][]string)
	if len(assocs) > 0 {
		var assocIDs []uuid.UUID
		for _, a := range assocs {
			assocIDs = append(assocIDs, a.ID)
		}
		var ops []AssociationOperation
		h.H.WithContext(ctx).
			Model(&AssociationOperation{}).
			Where("tenant_id = ? AND association_id IN ?", tenantID, assocIDs).
			Find(&ops)
		for _, op := range ops {
			opsMap[op.AssociationID] = append(opsMap[op.AssociationID], op.Operation)
		}
	}

	// Build result with operations
	results := make([]map[string]interface{}, len(assocs))
	for i, a := range assocs {
		result := map[string]interface{}{
			"id":         a.ID,
			"operations": opsMap[a.ID],
			"created_at": a.CreatedAt,
		}
		// Set subject side - exactly one must be set
		// Note: "subject-set" is API terminology only for the /api/v1/subject-sets endpoint.
		// In rules API, we use "subject-attribute" to refer to SubjectAttribute entities.
		if a.SubjectID != nil {
			result["subject_type"] = "subject"
			result["subject_id"] = *a.SubjectID
		} else if a.SubjectAttributeID != nil {
			result["subject_type"] = "subject-attribute"
			result["subject_id"] = *a.SubjectAttributeID
		} else {
			// Invalid state - skip this association
			continue
		}
		// Set object side - exactly one must be set
		// Note: "object-set" is API terminology only for the /api/v1/object-sets endpoint.
		// In rules API, we use "object-attribute" to refer to ObjectAttribute entities.
		if a.ObjectID != nil {
			result["object_type"] = "object"
			result["object_id"] = *a.ObjectID
		} else if a.ObjectAttributeID != nil {
			result["object_type"] = "object-attribute"
			result["object_id"] = *a.ObjectAttributeID
		} else {
			// Invalid state - skip this association
			continue
		}
		results[i] = result
	}

	nextCursor := ""
	if len(assocs) > 0 {
		nextCursor = assocs[len(assocs)-1].ID.String()
	}

	return assocs, results, nextCursor, hasMore, nil
}

// GetRule returns an association with operations
func (h *Handler) GetRule(ctx context.Context, tenantID string, assocID uuid.UUID) (*Association, []string, error) {
	var assoc Association
	err := h.H.WithContext(ctx).
		Model(&Association{}).
		Where("tenant_id = ? AND id = ?", tenantID, assocID).
		First(&assoc).Error
	if err != nil {
		return nil, nil, err
	}

	var ops []AssociationOperation
	h.H.WithContext(ctx).
		Model(&AssociationOperation{}).
		Where("tenant_id = ? AND association_id = ?", tenantID, assocID).
		Find(&ops)

	operations := make([]string, len(ops))
	for i, op := range ops {
		operations[i] = op.Operation
	}

	return &assoc, operations, nil
}

// GetRuleSubjectType returns the type and ID of the subject side of a rule
// GetSubjectType returns the type and ID of the subject side of a rule.
// Note: "subject-set" is API terminology only for the /api/v1/subject-sets endpoint.
// In rules API, we use "subject-attribute" to refer to SubjectAttribute entities.
func (a *Association) GetSubjectType() (string, uuid.UUID) {
	if a.SubjectID != nil {
		return "subject", *a.SubjectID
	} else if a.SubjectAttributeID != nil {
		return "subject-attribute", *a.SubjectAttributeID
	}
	return "", uuid.Nil
}

// GetObjectType returns the type and ID of the object side of a rule.
// Note: "object-set" is API terminology only for the /api/v1/object-sets endpoint.
// In rules API, we use "object-attribute" to refer to ObjectAttribute entities.
func (a *Association) GetObjectType() (string, uuid.UUID) {
	if a.ObjectID != nil {
		return "object", *a.ObjectID
	} else if a.ObjectAttributeID != nil {
		return "object-attribute", *a.ObjectAttributeID
	}
	return "", uuid.Nil
}

// GetAssociationsByUAOA returns all associations matching UA->OA pairs for a given operation
func (h *Handler) GetAssociationsByUAOA(ctx context.Context, tenantID string, uaOAPairs map[uuid.UUID][]uuid.UUID, operation string) ([]Association, error) {
	if len(uaOAPairs) == 0 {
		return []Association{}, nil
	}

	// First, get all association IDs that have the requested operation
	var assocOps []AssociationOperation
	err := h.H.WithContext(ctx).
		Model(&AssociationOperation{}).
		Where("tenant_id = ? AND operation = ?", tenantID, operation).
		Find(&assocOps).Error
	if err != nil {
		return nil, err
	}

	// Get association IDs that have this operation
	assocIDSet := make(map[uuid.UUID]bool)
	for _, ao := range assocOps {
		assocIDSet[ao.AssociationID] = true
	}

	if len(assocIDSet) == 0 {
		return []Association{}, nil
	}

	// Build list of association IDs to query
	assocIDs := make([]uuid.UUID, 0, len(assocIDSet))
	for id := range assocIDSet {
		assocIDs = append(assocIDs, id)
	}

	// Query associations by IDs
	var assocs []Association
	err = h.H.WithContext(ctx).
		Model(&Association{}).
		Where("tenant_id = ? AND id IN ?", tenantID, assocIDs).
		Find(&assocs).Error
	if err != nil {
		return nil, err
	}

	// Filter to only associations that match the UA->OA pairs
	// This function is specifically for UA->OA associations used by the engine
	filtered := []Association{}
	for _, a := range assocs {
		// Only process UA->OA associations (both attribute IDs must be set)
		if a.SubjectAttributeID == nil || a.ObjectAttributeID == nil {
			continue
		}
		if oaIDs, ok := uaOAPairs[*a.SubjectAttributeID]; ok {
			// Check if this association's OA is in the list
			for _, oaID := range oaIDs {
				if *a.ObjectAttributeID == oaID {
					filtered = append(filtered, a)
					break
				}
			}
		}
	}

	return filtered, nil
}

// GetProhibitionsBySubjectOA returns all prohibitions matching subject->OA pairs for a given operation
func (h *Handler) GetProhibitionsBySubjectOA(ctx context.Context, tenantID string, subjectMatches map[uuid.UUID][]uuid.UUID, uaMatches map[uuid.UUID][]uuid.UUID, operation string) ([]Prohibition, error) {
	if len(subjectMatches) == 0 && len(uaMatches) == 0 {
		return []Prohibition{}, nil
	}

	// First, get all prohibition IDs that have the requested operation
	var prohOps []ProhibitionOperation
	err := h.H.WithContext(ctx).
		Model(&ProhibitionOperation{}).
		Where("tenant_id = ? AND operation = ?", tenantID, operation).
		Find(&prohOps).Error
	if err != nil {
		return nil, err
	}

	// Get prohibition IDs that have this operation
	prohIDSet := make(map[uuid.UUID]bool)
	for _, po := range prohOps {
		prohIDSet[po.ProhibitionID] = true
	}

	if len(prohIDSet) == 0 {
		return []Prohibition{}, nil
	}

	// Build list of prohibition IDs to query
	prohIDs := make([]uuid.UUID, 0, len(prohIDSet))
	for id := range prohIDSet {
		prohIDs = append(prohIDs, id)
	}

	// Query prohibitions by IDs
	var prohs []Prohibition
	err = h.H.WithContext(ctx).
		Model(&Prohibition{}).
		Where("tenant_id = ? AND id IN ?", tenantID, prohIDs).
		Find(&prohs).Error
	if err != nil {
		return nil, err
	}

	// Filter to only prohibitions that match the subject->OA pairs
	filtered := []Prohibition{}
	for _, p := range prohs {
		matched := false

		// Check subject-level prohibitions
		if p.SubjectType == ProhibitSubject {
			if oaIDs, ok := subjectMatches[p.SubjectID]; ok {
				for _, oaID := range oaIDs {
					if p.ObjectAttributeID == oaID {
						matched = true
						break
					}
				}
			}
		}

		// Check UA-level prohibitions
		if p.SubjectType == ProhibitUA {
			if oaIDs, ok := uaMatches[p.SubjectID]; ok {
				for _, oaID := range oaIDs {
					if p.ObjectAttributeID == oaID {
						matched = true
						break
					}
				}
			}
		}

		if matched {
			filtered = append(filtered, p)
		}
	}

	return filtered, nil
}

// UpdateRule updates operations for an association
func (h *Handler) UpdateRule(ctx context.Context, tenantID string, assocID uuid.UUID, ops []string) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var assoc Association
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, assocID).First(&assoc).Error; err != nil {
			return err
		}

		// Delete existing operations
		if err := tx.Where("tenant_id = ? AND association_id = ?", tenantID, assocID).
			Delete(&AssociationOperation{}).Error; err != nil {
			return err
		}

		// Add new operations
		for _, op := range ops {
			ao := AssociationOperation{
				TenantID:      tenantID,
				AssociationID: assocID,
				Operation:     op,
			}
			if err := tx.Create(&ao).Error; err != nil {
				return err
			}
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"ua_id": assoc.SubjectAttributeID,
			"oa_id": assoc.ObjectAttributeID,
			"ops":   ops,
		}
		return AppendChange(tx, tenantID, rev, "ASSOC_OP", OpUpdate, payload)
	})
}

// ListDenies returns paginated list of prohibitions with operations
func (h *Handler) ListDenies(ctx context.Context, tenantID string, filters map[string]interface{}, limit int, cursor string) ([]Prohibition, []map[string]interface{}, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Model(&Prohibition{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if subjectType, ok := filters["subject_type"].(string); ok && subjectType != "" {
		db = db.Where("subject_type = ?", ProhibitionSubjectType(subjectType))
	}

	if subjectID, ok := filters["subject_id"].(uuid.UUID); ok && subjectID != uuid.Nil {
		db = db.Where("subject_id = ?", subjectID)
	}

	if oaID, ok := filters["target_id"].(uuid.UUID); ok && oaID != uuid.Nil {
		db = db.Where("object_attribute_id = ?", oaID)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var prohs []Prohibition
	err := db.Order("id ASC").Limit(limit + 1).Find(&prohs).Error
	if err != nil {
		return nil, nil, "", false, err
	}

	hasMore := len(prohs) > limit
	if hasMore {
		prohs = prohs[:limit]
	}

	// Load operations for each prohibition
	opsMap := make(map[uuid.UUID][]string)
	if len(prohs) > 0 {
		var prohIDs []uuid.UUID
		for _, p := range prohs {
			prohIDs = append(prohIDs, p.ID)
		}
		var ops []ProhibitionOperation
		h.H.WithContext(ctx).
			Model(&ProhibitionOperation{}).
			Where("tenant_id = ? AND prohibition_id IN ?", tenantID, prohIDs).
			Find(&ops)
		for _, op := range ops {
			opsMap[op.ProhibitionID] = append(opsMap[op.ProhibitionID], op.Operation)
		}
	}

	// Build result with operations
	results := make([]map[string]interface{}, len(prohs))
	for i, p := range prohs {
		results[i] = map[string]interface{}{
			"id":           p.ID,
			"subject_type": p.SubjectType,
			"subject_id":   p.SubjectID,
			"oa_id":        p.ObjectAttributeID,
			"operations":   opsMap[p.ID],
			"created_at":   p.CreatedAt,
		}
	}

	nextCursor := ""
	if len(prohs) > 0 {
		nextCursor = prohs[len(prohs)-1].ID.String()
	}

	return prohs, results, nextCursor, hasMore, nil
}

// GetDeny returns a prohibition with operations
func (h *Handler) GetDeny(ctx context.Context, tenantID string, prohID uuid.UUID) (*Prohibition, []string, error) {
	var proh Prohibition
	err := h.H.WithContext(ctx).
		Model(&Prohibition{}).
		Where("tenant_id = ? AND id = ?", tenantID, prohID).
		First(&proh).Error
	if err != nil {
		return nil, nil, err
	}

	var ops []ProhibitionOperation
	h.H.WithContext(ctx).
		Model(&ProhibitionOperation{}).
		Where("tenant_id = ? AND prohibition_id = ?", tenantID, prohID).
		Find(&ops)

	operations := make([]string, len(ops))
	for i, op := range ops {
		operations[i] = op.Operation
	}

	return &proh, operations, nil
}

// UpdateDeny updates operations and targets for a prohibition
func (h *Handler) UpdateDeny(ctx context.Context, tenantID string, prohID uuid.UUID, ops []string, oaIDs []uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var proh Prohibition
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, prohID).First(&proh).Error; err != nil {
			return err
		}

		// Update target OA if provided
		if len(oaIDs) > 0 {
			// For simplicity, update to first OA (could support multiple targets later)
			tx.Model(&proh).Update("object_attribute_id", oaIDs[0])
		}

		// Delete existing operations
		if err := tx.Where("tenant_id = ? AND prohibition_id = ?", tenantID, prohID).
			Delete(&ProhibitionOperation{}).Error; err != nil {
			return err
		}

		// Add new operations
		for _, op := range ops {
			po := ProhibitionOperation{
				TenantID:      tenantID,
				ProhibitionID: prohID,
				Operation:     op,
			}
			if err := tx.Create(&po).Error; err != nil {
				return err
			}
		}

		rev, _ := BumpRevision(tx, tenantID)
		payload := map[string]any{
			"subject_type": proh.SubjectType,
			"subject_id":   proh.SubjectID,
			"oa_id":        proh.ObjectAttributeID,
			"op":           ops,
		}
		return AppendChange(tx, tenantID, rev, "PROHIB_OP", OpUpdate, payload)
	})
}

// GetGraphSummary returns counts of all entities
func (h *Handler) GetGraphSummary(ctx context.Context, tenantID string) (map[string]int64, error) {
	summary := make(map[string]int64)

	var count int64
	h.H.WithContext(ctx).Model(&Subject{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["subjects"] = count

	h.H.WithContext(ctx).Model(&Object{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["objects"] = count

	h.H.WithContext(ctx).Model(&SubjectAttribute{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["subject_sets"] = count

	h.H.WithContext(ctx).Model(&ObjectAttribute{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["object_sets"] = count

	h.H.WithContext(ctx).Model(&AssignmentEdge{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["relationships"] = count

	h.H.WithContext(ctx).Model(&Association{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["rules"] = count

	h.H.WithContext(ctx).Model(&Prohibition{}).Where("tenant_id = ?", tenantID).Count(&count)
	summary["denies"] = count

	return summary, nil
}

// GraphSearch searches for nodes by name/external_id
func (h *Handler) GraphSearch(ctx context.Context, tenantID string, query string, types []string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	if query == "" {
		return []map[string]interface{}{}, nil
	}

	search := "%" + query + "%"
	results := []map[string]interface{}{}

	typeSet := make(map[string]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	// Search subjects
	if len(types) == 0 || typeSet["subject"] {
		var subjects []Subject
		h.H.WithContext(ctx).
			Model(&Subject{}).
			Where("tenant_id = ? AND (external_id ILIKE ? OR email ILIKE ? OR display ILIKE ?)", tenantID, search, search, search).
			Limit(limit).
			Find(&subjects)
		for _, s := range subjects {
			results = append(results, map[string]interface{}{
				"id":   s.ID,
				"type": "subject",
				"name": s.ExternalID,
			})
		}
	}

	// Search UAs (SubjectAttributes)
	// Accept both "subject-set" (legacy) and "subject-attribute" (new) for backward compatibility
	// But return "subject-attribute" in results for consistency
	if len(types) == 0 || typeSet["subject-set"] || typeSet["subject-attribute"] || typeSet["subject-group"] {
		var uas []SubjectAttribute
		h.H.WithContext(ctx).
			Model(&SubjectAttribute{}).
			Where("tenant_id = ? AND name ILIKE ?", tenantID, search).
			Limit(limit).
			Find(&uas)
		for _, ua := range uas {
			results = append(results, map[string]interface{}{
				"id":   ua.ID,
				"type": "subject-attribute", // Use "subject-attribute" for consistency (not "subject-set")
				"name": ua.Name,
			})
		}
	}

	// Search objects
	if len(types) == 0 || typeSet["object"] {
		var objects []Object
		h.H.WithContext(ctx).
			Model(&Object{}).
			Where("tenant_id = ? AND (external_id ILIKE ? OR type ILIKE ?)", tenantID, search, search).
			Limit(limit).
			Find(&objects)
		for _, o := range objects {
			results = append(results, map[string]interface{}{
				"id":   o.ID,
				"type": "object",
				"name": o.ExternalID,
			})
		}
	}

	// Search OAs (ObjectAttributes)
	// Accept both "object-set" (legacy) and "object-attribute" (new) for backward compatibility
	// But return "object-attribute" in results for consistency
	if len(types) == 0 || typeSet["object-set"] || typeSet["object-attribute"] || typeSet["object-group"] {
		var oas []ObjectAttribute
		h.H.WithContext(ctx).
			Model(&ObjectAttribute{}).
			Where("tenant_id = ? AND name ILIKE ?", tenantID, search).
			Limit(limit).
			Find(&oas)
		for _, oa := range oas {
			results = append(results, map[string]interface{}{
				"id":   oa.ID,
				"type": "object-attribute", // Use "object-attribute" for consistency (not "object-set")
				"name": oa.Name,
			})
		}
	}

	return results, nil
}
