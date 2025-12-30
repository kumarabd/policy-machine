package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListSubjects returns paginated list of subjects
func (h *Handler) ListSubjects(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]Subject, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

	// Apply search query
	if query != "" {
		search := "%" + query + "%"
		db = db.Where("external_id ILIKE ? OR email ILIKE ? OR display ILIKE ?", search, search, search)
	}

	// Apply cursor (simple: use ID > cursor)
	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var subjects []Subject
	err := db.Order("id ASC").Limit(limit + 1).Find(&subjects).Error
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

// ListSubjectSets returns paginated list of subject attributes
func (h *Handler) ListSubjectSets(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]UserAttribute, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

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

	var uas []UserAttribute
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

// Backward compatibility aliases
func (h *Handler) ListSubjectGroups(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]UserAttribute, string, bool, error) {
	return h.ListSubjectSets(ctx, tenantID, query, limit, cursor)
}

func (h *Handler) GetSubjectGroup(ctx context.Context, tenantID string, uaID uuid.UUID) (*UserAttribute, error) {
	return h.GetSubjectSet(ctx, tenantID, uaID)
}

func (h *Handler) UpdateSubjectGroup(ctx context.Context, tenantID string, uaID uuid.UUID, name string) (int64, error) {
	return h.UpdateSubjectSet(ctx, tenantID, uaID, name)
}

func (h *Handler) DeleteSubjectGroup(ctx context.Context, tenantID string, uaID uuid.UUID) (int64, error) {
	return h.DeleteSubjectSet(ctx, tenantID, uaID)
}

// GetSubjectSet returns a subject attribute by ID
func (h *Handler) GetSubjectSet(ctx context.Context, tenantID string, uaID uuid.UUID) (*UserAttribute, error) {
	var ua UserAttribute
	err := h.H.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, uaID).
		First(&ua).Error
	if err != nil {
		return nil, err
	}
	return &ua, nil
}

// GetSubjectSetMembers returns all subject IDs that are members of a subject set
func (h *Handler) GetSubjectSetMembers(ctx context.Context, tenantID string, uaID uuid.UUID) ([]uuid.UUID, error) {
	var edges []AssignmentEdge
	err := h.H.WithContext(ctx).
		Where("tenant_id = ? AND parent_type = ? AND parent_id = ? AND child_type = ?", tenantID, NodeUA, uaID, NodeUser).
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

// UpdateSubjectSet updates a subject attribute
func (h *Handler) UpdateSubjectSet(ctx context.Context, tenantID string, uaID uuid.UUID, name string) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		result := tx.Model(&UserAttribute{}).
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

// DeleteSubjectSet deletes a subject attribute
func (h *Handler) DeleteSubjectSet(ctx context.Context, tenantID string, uaID uuid.UUID) (int64, error) {
	return h.WithPolicyWriteTx(ctx, tenantID, func(tx *gorm.DB) error {
		var ua UserAttribute
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

// ListObjects returns paginated list of objects
func (h *Handler) ListObjects(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]Object, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if query != "" {
		search := "%" + query + "%"
		db = db.Where("external_id ILIKE ? OR type ILIKE ?", search, search)
	}

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			db = db.Where("id > ?", cursorID)
		}
	}

	var objects []Object
	err := db.Order("id ASC").Limit(limit + 1).Find(&objects).Error
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

// ListObjectSets returns paginated list of object attributes
func (h *Handler) ListObjectSets(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]ObjectAttribute, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

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

// Backward compatibility aliases for ObjectSets
func (h *Handler) ListObjectGroups(ctx context.Context, tenantID string, query string, limit int, cursor string) ([]ObjectAttribute, string, bool, error) {
	return h.ListObjectSets(ctx, tenantID, query, limit, cursor)
}

func (h *Handler) GetObjectGroup(ctx context.Context, tenantID string, oaID uuid.UUID) (*ObjectAttribute, error) {
	return h.GetObjectSet(ctx, tenantID, oaID)
}

func (h *Handler) UpdateObjectGroup(ctx context.Context, tenantID string, oaID uuid.UUID, name string) (int64, error) {
	return h.UpdateObjectSet(ctx, tenantID, oaID, name)
}

func (h *Handler) DeleteObjectGroup(ctx context.Context, tenantID string, oaID uuid.UUID) (int64, error) {
	return h.DeleteObjectSet(ctx, tenantID, oaID)
}

// GetObjectSet returns an object attribute by ID
func (h *Handler) GetObjectSet(ctx context.Context, tenantID string, oaID uuid.UUID) (*ObjectAttribute, error) {
	var oa ObjectAttribute
	err := h.H.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, oaID).
		First(&oa).Error
	if err != nil {
		return nil, err
	}
	return &oa, nil
}

// GetObjectSetMembers returns all object IDs that are members of an object set
func (h *Handler) GetObjectSetMembers(ctx context.Context, tenantID string, oaID uuid.UUID) ([]uuid.UUID, error) {
	var edges []AssignmentEdge
	err := h.H.WithContext(ctx).
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

// UpdateObjectSet updates an object attribute
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

// DeleteObjectSet deletes an object attribute
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

// MapRelationshipKindToEdgeTypes converts UI relationship kind to NGAC edge types
func MapRelationshipKindToEdgeTypes(kind string, fromType, toType string) (childType, parentType NodeType, err error) {
	switch kind {
	case "subject_member_of_set", "subject_member_of_group":
		if fromType != "subject" || (toType != "subject-set" && toType != "subject-group") {
			return "", "", fmt.Errorf("invalid types for subject_member_of_set")
		}
		return NodeUser, NodeUA, nil
	case "subject_set_parent_of_set", "subject_group_parent_of_group":
		if (fromType != "subject-set" && fromType != "subject-group") || (toType != "subject-set" && toType != "subject-group") {
			return "", "", fmt.Errorf("invalid types for subject_set_parent_of_set")
		}
		return NodeUA, NodeUA, nil
	case "object_member_of_set", "object_member_of_group":
		if fromType != "object" || (toType != "object-set" && toType != "object-group") {
			return "", "", fmt.Errorf("invalid types for object_member_of_set")
		}
		return NodeObject, NodeOA, nil
	case "object_set_parent_of_set", "object_group_parent_of_group":
		if (fromType != "object-set" && fromType != "object-group") || (toType != "object-set" && toType != "object-group") {
			return "", "", fmt.Errorf("invalid types for object_set_parent_of_set")
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

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

	// Apply filters
	if kind, ok := filters["kind"].(string); ok && kind != "" {
		// Filter by child/parent type based on kind
		switch kind {
		case "subject_member_of_set", "subject_member_of_group":
			db = db.Where("child_type = ? AND parent_type = ?", NodeUser, NodeUA)
		case "subject_set_parent_of_set", "subject_group_parent_of_group":
			db = db.Where("child_type = ? AND parent_type = ?", NodeUA, NodeUA)
		case "object_member_of_set", "object_member_of_group":
			db = db.Where("child_type = ? AND parent_type = ?", NodeObject, NodeOA)
		case "object_set_parent_of_set", "object_group_parent_of_group":
			db = db.Where("child_type = ? AND parent_type = ?", NodeOA, NodeOA)
		}
	}

	if fromType, ok := filters["from_type"].(string); ok && fromType != "" {
		// Map UI type to NodeType
		var nodeType NodeType
		switch fromType {
		case "subject":
			nodeType = NodeUser
		case "subject-group":
			nodeType = NodeUA
		case "object":
			nodeType = NodeObject
		case "object-set", "object-group":
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
		var nodeType NodeType
		switch toType {
		case "subject-set", "subject-group":
			nodeType = NodeUA
		case "object-set", "object-group":
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

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

	// Apply filters
	if uaID, ok := filters["subject_scope_id"].(uuid.UUID); ok && uaID != uuid.Nil {
		db = db.Where("user_attribute_id = ?", uaID)
	}

	if oaID, ok := filters["object_scope_id"].(uuid.UUID); ok && oaID != uuid.Nil {
		db = db.Where("object_attribute_id = ?", oaID)
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
			Where("tenant_id = ? AND association_id IN ?", tenantID, assocIDs).
			Find(&ops)
		for _, op := range ops {
			opsMap[op.AssociationID] = append(opsMap[op.AssociationID], op.Operation)
		}
	}

	// Build result with operations
	results := make([]map[string]interface{}, len(assocs))
	for i, a := range assocs {
		results[i] = map[string]interface{}{
			"id":         a.ID,
			"ua_id":      a.UserAttributeID,
			"oa_id":      a.ObjectAttributeID,
			"operations": opsMap[a.ID],
			"created_at": a.CreatedAt,
		}
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
		Where("tenant_id = ? AND id = ?", tenantID, assocID).
		First(&assoc).Error
	if err != nil {
		return nil, nil, err
	}

	var ops []AssociationOperation
	h.H.WithContext(ctx).
		Where("tenant_id = ? AND association_id = ?", tenantID, assocID).
		Find(&ops)

	operations := make([]string, len(ops))
	for i, op := range ops {
		operations[i] = op.Operation
	}

	return &assoc, operations, nil
}

// GetAssociationsByUAOA returns all associations matching UA->OA pairs for a given operation
func (h *Handler) GetAssociationsByUAOA(ctx context.Context, tenantID string, uaOAPairs map[uuid.UUID][]uuid.UUID, operation string) ([]Association, error) {
	if len(uaOAPairs) == 0 {
		return []Association{}, nil
	}

	// First, get all association IDs that have the requested operation
	var assocOps []AssociationOperation
	err := h.H.WithContext(ctx).
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
		Where("tenant_id = ? AND id IN ?", tenantID, assocIDs).
		Find(&assocs).Error
	if err != nil {
		return nil, err
	}

	// Filter to only associations that match the UA->OA pairs
	filtered := []Association{}
	for _, a := range assocs {
		if oaIDs, ok := uaOAPairs[a.UserAttributeID]; ok {
			// Check if this association's OA is in the list
			for _, oaID := range oaIDs {
				if a.ObjectAttributeID == oaID {
					filtered = append(filtered, a)
					break
				}
			}
		}
	}

	return filtered, nil
}

// GetProhibitionsBySubjectOA returns all prohibitions matching subject->OA pairs for a given operation
func (h *Handler) GetProhibitionsBySubjectOA(ctx context.Context, tenantID string, userMatches map[uuid.UUID][]uuid.UUID, uaMatches map[uuid.UUID][]uuid.UUID, operation string) ([]Prohibition, error) {
	if len(userMatches) == 0 && len(uaMatches) == 0 {
		return []Prohibition{}, nil
	}

	// First, get all prohibition IDs that have the requested operation
	var prohOps []ProhibitionOperation
	err := h.H.WithContext(ctx).
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
		Where("tenant_id = ? AND id IN ?", tenantID, prohIDs).
		Find(&prohs).Error
	if err != nil {
		return nil, err
	}

	// Filter to only prohibitions that match the subject->OA pairs
	filtered := []Prohibition{}
	for _, p := range prohs {
		matched := false

		// Check user-level prohibitions
		if p.SubjectType == ProhibitUser {
			if oaIDs, ok := userMatches[p.SubjectID]; ok {
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
			"ua_id": assoc.UserAttributeID,
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

	db := h.H.WithContext(ctx).Where("tenant_id = ?", tenantID)

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
		Where("tenant_id = ? AND id = ?", tenantID, prohID).
		First(&proh).Error
	if err != nil {
		return nil, nil, err
	}

	var ops []ProhibitionOperation
	h.H.WithContext(ctx).
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

	h.H.WithContext(ctx).Model(&UserAttribute{}).Where("tenant_id = ?", tenantID).Count(&count)
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

	// Search UAs
	if len(types) == 0 || typeSet["subject-set"] || typeSet["subject-group"] {
		var uas []UserAttribute
		h.H.WithContext(ctx).
			Where("tenant_id = ? AND name ILIKE ?", tenantID, search).
			Limit(limit).
			Find(&uas)
		for _, ua := range uas {
			results = append(results, map[string]interface{}{
				"id":   ua.ID,
				"type": "subject-set",
				"name": ua.Name,
			})
		}
	}

	// Search objects
	if len(types) == 0 || typeSet["object"] {
		var objects []Object
		h.H.WithContext(ctx).
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

	// Search OAs
	if len(types) == 0 || typeSet["object-set"] || typeSet["object-group"] {
		var oas []ObjectAttribute
		h.H.WithContext(ctx).
			Where("tenant_id = ? AND name ILIKE ?", tenantID, search).
			Limit(limit).
			Find(&oas)
		for _, oa := range oas {
			results = append(results, map[string]interface{}{
				"id":   oa.ID,
				"type": "object-set",
				"name": oa.Name,
			})
		}
	}

	return results, nil
}
