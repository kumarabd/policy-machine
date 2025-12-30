package validate

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ValidateNodeExistence ensures all referenced nodes exist in the database
func ValidateNodeExistence(ctx context.Context, tx *gorm.DB, tenantID string, edge AssignmentEdge) error {
	// Validate child node
	if err := validateNode(ctx, tx, tenantID, edge.GetChildType(), edge.GetChildID()); err != nil {
		return err
	}

	// Validate parent node
	if err := validateNode(ctx, tx, tenantID, edge.GetParentType(), edge.GetParentID()); err != nil {
		return err
	}

	return nil
}

func validateNode(ctx context.Context, tx *gorm.DB, tenantID string, nodeType string, nodeID uuid.UUID) error {
	switch nodeType {
	case "UA":
		var count int64
		if err := tx.WithContext(ctx).
			Table("user_attributes").
			Where("tenant_id = ? AND id = ?", tenantID, nodeID).
			Select("1").
			Limit(1).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ValidationError{
				Code:    CodeMissingNode,
				Message: fmt.Sprintf("user attribute (UA) %s does not exist for tenant %s", nodeID, tenantID),
			}
		}

	case "OA":
		var count int64
		if err := tx.WithContext(ctx).
			Table("object_attributes").
			Where("tenant_id = ? AND id = ?", tenantID, nodeID).
			Select("1").
			Limit(1).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ValidationError{
				Code:    CodeMissingNode,
				Message: fmt.Sprintf("object attribute (OA) %s does not exist for tenant %s", nodeID, tenantID),
			}
		}

	case "PC":
		var count int64
		if err := tx.WithContext(ctx).
			Table("policy_classes").
			Where("tenant_id = ? AND id = ?", tenantID, nodeID).
			Select("1").
			Limit(1).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ValidationError{
				Code:    CodeMissingNode,
				Message: fmt.Sprintf("policy class (PC) %s does not exist for tenant %s", nodeID, tenantID),
			}
		}

	case "USER", "OBJECT":
		// USER and OBJECT are external IDs - we don't validate them
		// as they may not exist in our tables
		return nil

	default:
		return ValidationError{
			Code:    CodeInvalidEdgeType,
			Message: fmt.Sprintf("unknown node type: %s", nodeType),
		}
	}

	return nil
}
