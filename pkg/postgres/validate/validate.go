package validate

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssignmentEdge interface to avoid import cycle
type AssignmentEdge interface {
	GetTenantID() uuid.UUID
	GetChildType() string
	GetChildID() uuid.UUID
	GetParentType() string
	GetParentID() uuid.UUID
	SetTenantID(uuid.UUID)
}

// ValidateAssignmentEdgeCreate validates an assignment edge before creation
func ValidateAssignmentEdgeCreate(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID, edge AssignmentEdge) error {
	// Ensure tenant ID is set
	edge.SetTenantID(tenantID)

	// Validate edge type
	if err := ValidateEdgeType(edge); err != nil {
		return err
	}

	// Validate node existence
	if err := ValidateNodeExistence(ctx, tx, tenantID, edge); err != nil {
		return err
	}

	// Check for duplicate edge (idempotent - return nil if exists)
	var existing struct {
		ID uuid.UUID
	}
	err := tx.WithContext(ctx).
		Table("assignment_edges").
		Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ? AND parent_id = ?",
			tenantID, edge.GetChildType(), edge.GetChildID(), edge.GetParentType(), edge.GetParentID()).
		Select("id").
		Limit(1).
		First(&existing).Error
	if err == nil {
		// Edge already exists - idempotent success
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Check for cycles (only for UA->UA and OA->OA)
	childType := NodeType(edge.GetChildType())
	parentType := NodeType(edge.GetParentType())
	if childType == NodeUA && parentType == NodeUA {
		hasCycle, err := WouldCreateCycle(ctx, tx, tenantID, childType, parentType, edge.GetChildID(), edge.GetParentID())
		if err != nil {
			return err
		}
		if hasCycle {
			return ValidationError{
				Code:    CodeCycleDetected,
				Message: fmt.Sprintf("UA->UA cycle detected: adding edge from UA %s to UA %s would create a cycle", edge.GetChildID(), edge.GetParentID()),
			}
		}
	}

	if childType == NodeOA && parentType == NodeOA {
		hasCycle, err := WouldCreateCycle(ctx, tx, tenantID, childType, parentType, edge.GetChildID(), edge.GetParentID())
		if err != nil {
			return err
		}
		if hasCycle {
			return ValidationError{
				Code:    CodeCycleDetected,
				Message: fmt.Sprintf("OA->OA cycle detected: adding edge from OA %s to OA %s would create a cycle", edge.GetChildID(), edge.GetParentID()),
			}
		}
	}

	return nil
}

// ValidateEdgeType validates that the edge type combination is allowed
func ValidateEdgeType(edge AssignmentEdge) error {
	validCombinations := map[string]bool{
		// USER -> UA
		"USER->UA": true,
		// UA -> UA
		"UA->UA": true,
		// OBJECT -> OA
		"OBJECT->OA": true,
		// OA -> OA
		"OA->OA": true,
		// UA -> POLICY_CLASS
		"UA->PC": true,
		// OA -> POLICY_CLASS
		"OA->PC": true,
	}

	key := edge.GetChildType() + "->" + edge.GetParentType()
	if !validCombinations[key] {
		return ValidationError{
			Code:    CodeInvalidEdgeType,
			Message: fmt.Sprintf("invalid edge type combination: %s -> %s", edge.GetChildType(), edge.GetParentType()),
		}
	}

	return nil
}

