package validate

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NodeType matches postgres.NodeType to avoid import cycle
type NodeType string

const (
	NodeUA NodeType = "UA"
	NodeOA NodeType = "OA"
)

// WouldCreateCycle checks if adding an edge would create a cycle in the graph
// Only applies to UA->UA and OA->OA edges
func WouldCreateCycle(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID,
	childType, parentType NodeType, childID, parentID uuid.UUID) (bool, error) {

	// Only check cycles for UA->UA and OA->OA
	if !((childType == NodeUA && parentType == NodeUA) ||
		(childType == NodeOA && parentType == NodeOA)) {
		return false, nil
	}

	// Self-loop is always a cycle
	if childID == parentID {
		return true, nil
	}

	// Use recursive CTE to check if parentID can reach childID
	// We're checking if parentID (the new parent) can reach childID (the new child)
	// If so, adding childID -> parentID would create a cycle
	var result struct {
		HasCycle bool
	}

	// Determine the node type string for the query
	nodeTypeStr := "UA"
	if childType == NodeOA {
		nodeTypeStr = "OA"
	}

	// Recursive CTE query:
	// Start from parentID and traverse up the graph (following child -> parent direction)
	// If we ever reach childID, there's a cycle
	// We're checking: can parentID reach childID? If yes, adding childID->parentID creates a cycle
	query := `
		WITH RECURSIVE reachable AS (
			-- Base case: start from parentID itself
			SELECT ?::uuid as node_id
			
			UNION
			
			-- Recursive case: find all parents of current nodes
			SELECT ae.parent_id
			FROM assignment_edges ae
			INNER JOIN reachable r ON ae.child_id = r.node_id
			WHERE ae.tenant_id = ?
				AND ae.child_type = ?
				AND ae.parent_type = ?
		)
		SELECT EXISTS(
			SELECT 1 
			FROM reachable 
			WHERE node_id = ?::uuid
		) as has_cycle
	`

	err := tx.WithContext(ctx).Raw(query,
		parentID,
		tenantID, nodeTypeStr, nodeTypeStr,
		childID).Scan(&result).Error

	if err != nil {
		return false, fmt.Errorf("cycle detection query failed: %w", err)
	}

	return result.HasCycle, nil
}

