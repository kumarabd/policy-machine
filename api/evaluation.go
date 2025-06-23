package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/model"
)

// Request represents an access request
type Request struct {
	Subject     *model.Subject    `json:"subject"`
	Resource    *model.Resource   `json:"resource"`
	Actions     []string          `json:"actions"`
	Context     map[string]string `json:"context,omitempty"`
	PolicyClass string            `json:"policy_class,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
}

// Response represents the response to an access request
type Response struct {
	Decision    *Decision     `json:"decision"`
	Request     *Request      `json:"request"`
	ProcessTime time.Duration `json:"process_time"`
	Error       string        `json:"error,omitempty"`
}

type Decision struct {
	Permit       bool                 `json:"permit"`
	Reason       string               `json:"reason,omitempty"`
	Obligations  []string             `json:"obligations,omitempty"`
	Conditions   []string             `json:"conditions,omitempty"`
	Timestamp    time.Time            `json:"timestamp"`
	PolicyPath   []*model.Entity      `json:"policy_path,omitempty"`
	Prohibitions []*model.Prohibition `json:"prohibitions,omitempty"`
}

type Path struct {
	Nodes       []model.Entity       `json:"nodes"`
	Edges       []model.Relationship `json:"edges"`
	Actions     []string             `json:"actions"`
	Obligations []string             `json:"obligations"`
	IsValid     bool                 `json:"is_valid"`
	Depth       int                  `json:"depth"`
}

// Subgraph represents a connected component from a starting node
type Subgraph struct {
	Nodes         map[string]model.Entity         // nodeID -> Entity
	Relationships map[string][]model.Relationship // sourceID -> []Relationships
	ReverseRels   map[string][]model.Relationship // targetID -> []Relationships
}

// Context holds the context for path computation
type Context struct {
	PolicyClass string            `json:"policy_class"`
	SessionID   string            `json:"session_id,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	RequestID   string            `json:"request_id,omitempty"`
	// Runtime context
	SubjectGraph  *Subgraph
	ResourceGraph *Subgraph
	TargetActions []string
}

type Evaluator struct {
	log       *logger.Handler
	datalayer DataHandler
}

func NewEvaluator(l *logger.Handler, d DataHandler) *Evaluator {
	return &Evaluator{
		log:       l,
		datalayer: d,
	}
}

func (h *Evaluator) EvaluatePolicy(ctx context.Context, req *Request) (*Decision, error) {
	startTime := time.Now()

	h.log.Debug().
		Str("subject", req.Subject.EntityID).
		Str("resource", req.Resource.EntityID).
		Strs("actions", req.Actions).
		Msg("Starting  access evaluation")

	privilegePaths, err := h.evaluatePathsUsingSubgraphs(req.PolicyClass, req.Subject.DeepCopy(), req.Resource.DeepCopy(), req.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to find privilege paths: %w", err)
	}

	h.log.Debug().
		Int("privilege_paths", len(privilegePaths)).
		Msg("Found privilege paths")

	// If no privilege paths found, access is denied
	if len(privilegePaths) == 0 {
		return &Decision{
			Permit:    false,
			Reason:    "No privilege path found from subject to resource",
			Timestamp: time.Now(),
		}, nil
	}

	// Step 2: Check for applicable prohibitions
	prohibitions, err := h.checkProhibitionsForRequest(ctx, req, privilegePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to check prohibitions: %w", err)
	}

	h.log.Debug().
		Int("prohibitions", len(prohibitions)).
		Msg("Checked prohibitions")

	// If any prohibitions apply, access is denied
	if len(prohibitions) > 0 {
		return &Decision{
			Permit:       false,
			Reason:       fmt.Sprintf("Access prohibited by %d prohibition(s)", len(prohibitions)),
			Timestamp:    time.Now(),
			Prohibitions: prohibitions,
		}, nil
	}

	// Step 3: Extract obligations from privilege paths
	obligations := h.extractObligations(privilegePaths)

	// Access is granted
	decision := &Decision{
		Permit:      true,
		Reason:      fmt.Sprintf("Access granted via %d privilege path(s)", len(privilegePaths)),
		Obligations: obligations,
		Timestamp:   time.Now(),
		PolicyPath:  h.extractPolicyPath(privilegePaths),
	}

	// // Update statistics
	// h.mutex.Lock()
	// h.stats.EvaluationTime = time.Since(startTime)
	// h.stats.PathsEvaluated += len(privilegePaths)
	// h.stats.ProhibitionsChecked += len(prohibitions)
	// h.mutex.Unlock()

	h.log.Debug().
		Bool("permit", decision.Permit).
		Dur("evaluation_time", time.Since(startTime)).
		Msg("Completed  access evaluation")

	return decision, nil
}

// evaluatePathsUsingSubgraphs implements the new efficient subgraph-based algorithm
func (h *Evaluator) evaluatePathsUsingSubgraphs(class string, subjectEntity model.Entity, resourceEntity model.Entity, targetActions []string) ([]*Path, error) {
	h.log.Debug().Msgf("Starting subgraph-based evaluation from %s to %s for actions %v", subjectEntity.HashID, resourceEntity.HashID, targetActions)

	// Step 1 & 2: Build subgraphs concurrently
	var subjectGraph, resourceGraph *Subgraph
	var subjectErr, resourceErr error
	var wg sync.WaitGroup

	wg.Add(2)

	// Build subject subgraph
	go func() {
		defer wg.Done()
		subjectGraph, subjectErr = h.buildSubgraph(subjectEntity.HashID, "subject")
	}()

	// Build resource subgraph
	go func() {
		defer wg.Done()
		resourceGraph, resourceErr = h.buildSubgraph(resourceEntity.HashID, "resource")
	}()

	wg.Wait()

	if subjectErr != nil {
		return nil, fmt.Errorf("error building subject subgraph: %w", subjectErr)
	}
	if resourceErr != nil {
		return nil, fmt.Errorf("error building resource subgraph: %w", resourceErr)
	}

	h.log.Debug().Msgf("Built subgraphs - Subject: %d nodes, Resource: %d nodes",
		len(subjectGraph.Nodes), len(resourceGraph.Nodes))

	// Step 3-7: Find paths through association intersections
	context := &Context{
		PolicyClass:   class,
		SubjectGraph:  subjectGraph,
		ResourceGraph: resourceGraph,
		TargetActions: targetActions,
	}

	return h.findPathsThroughIntersections(context, subjectEntity, resourceEntity)
}

// buildSubgraph builds a subgraph starting from the given node using BFS
func (h *Evaluator) buildSubgraph(startNodeID string, graphType string) (*Subgraph, error) {
	h.log.Debug().Msgf("Building %s subgraph from node %s", graphType, startNodeID)

	subgraph := &Subgraph{
		Nodes:         make(map[string]model.Entity),
		Relationships: make(map[string][]model.Relationship),
		ReverseRels:   make(map[string][]model.Relationship),
	}

	visited := make(map[string]bool)
	queue := []string{startNodeID}
	visited[startNodeID] = true
	maxDepth := 10
	currentDepth := 0

	// Get the starting entity
	startEntity := &model.Entity{}
	if err := h.datalayer.FetchEntityForID(startNodeID, startEntity); err != nil {
		return nil, fmt.Errorf("error fetching start entity %s: %w", startNodeID, err)
	}
	subgraph.Nodes[startNodeID] = *startEntity

	for len(queue) > 0 && currentDepth < maxDepth {
		currentLevelSize := len(queue)

		for i := 0; i < currentLevelSize; i++ {
			currentNodeID := queue[0]
			queue = queue[1:]

			// Fetch all relationships where current node is the source
			relationships := make([]model.Relationship, 0)
			if err := h.datalayer.FetchRelationshipsForSource(currentNodeID, &relationships); err != nil {
				h.log.Debug().Msgf("Error fetching relationships for %s: %v", currentNodeID, err)
				continue
			}

			subgraph.Relationships[currentNodeID] = relationships

			for _, rel := range relationships {
				// Add to reverse relationships
				if subgraph.ReverseRels[rel.ToID] == nil {
					subgraph.ReverseRels[rel.ToID] = make([]model.Relationship, 0)
				}
				subgraph.ReverseRels[rel.ToID] = append(subgraph.ReverseRels[rel.ToID], rel)

				// Add target entity to subgraph if not already visited
				if !visited[rel.ToID] {
					targetEntity := &model.Entity{}
					if err := h.datalayer.FetchEntityForID(rel.ToID, targetEntity); err != nil {
						continue // Skip if entity not found
					}

					subgraph.Nodes[rel.ToID] = *targetEntity
					visited[rel.ToID] = true
					queue = append(queue, rel.ToID)
				}
			}
		}
		currentDepth++
	}

	h.log.Debug().Msgf("Built %s subgraph with %d nodes and %d relationship sources",
		graphType, len(subgraph.Nodes), len(subgraph.Relationships))

	return subgraph, nil
}

// findPathsThroughIntersections finds paths by identifying intersection points between subgraphs
func (h *Evaluator) findPathsThroughIntersections(context *Context, subjectEntity model.Entity, resourceEntity model.Entity) ([]*Path, error) {
	h.log.Debug().Msgf("Finding paths through subgraph intersections")

	var validPaths []*Path

	// Step 3: Find nodes in subject subgraph that have association relationships
	associationNodes := h.findNodesWithAssociations(context.SubjectGraph)
	h.log.Debug().Msgf("Found %d nodes with associations in subject subgraph", len(associationNodes))

	// Step 4-6: Check if association targets are in resource subgraph
	intersectionPoints := make(map[string][]*model.Association) // targetNodeID -> []*Association

	for _, nodeID := range associationNodes {
		associations := h.getAssociationsForNode(context.PolicyClass, nodeID, context.SubjectGraph)

		for _, assoc := range associations {
			// Step 5: Check if ToID is in resource subgraph
			if _, exists := context.ResourceGraph.Nodes[assoc.Relationship.ToID]; exists {
				h.log.Debug().Msgf("Found intersection: association from %s to %s (in resource subgraph)",
					nodeID, assoc.Relationship.ToID)

				if intersectionPoints[assoc.Relationship.ToID] == nil {
					intersectionPoints[assoc.Relationship.ToID] = make([]*model.Association, 0)
				}
				intersectionPoints[assoc.Relationship.ToID] = append(intersectionPoints[assoc.Relationship.ToID], assoc)
			}
		}
	}

	h.log.Debug().Msgf("Found %d intersection points", len(intersectionPoints))

	// Step 7: Compute paths through each intersection point
	for intersectionNodeID, associations := range intersectionPoints {
		// For each association leading to this intersection point
		for _, assoc := range associations {
			// Check if this association has the required actions
			if h.associationHasRequiredActions(assoc, context.TargetActions) {
				// Build path from subject to association source
				subjectToAssocSource := h.findPathInSubgraph(context.PolicyClass, context.SubjectGraph, subjectEntity.HashID, assoc.Relationship.FromID)

				// Build path from intersection point to resource
				intersectionToResource := h.findPathInSubgraph(context.PolicyClass, context.ResourceGraph, resourceEntity.HashID, intersectionNodeID)

				if subjectToAssocSource != nil && intersectionToResource != nil {
					// Combine the paths
					completePath := h.combinePaths(subjectToAssocSource, assoc, intersectionToResource)
					if completePath != nil {
						validPaths = append(validPaths, completePath)
						h.log.Debug().Msgf("Valid path found through intersection %s with %d total nodes",
							intersectionNodeID, len(completePath.Nodes))
					}
				}
			}
		}
	}

	// If no intersection paths found, check for direct connection
	if len(validPaths) == 0 {
		if _, subjectHasResource := context.SubjectGraph.Nodes[resourceEntity.HashID]; subjectHasResource {
			directPath := h.findPathInSubgraph(context.PolicyClass, context.SubjectGraph, subjectEntity.HashID, resourceEntity.HashID)
			if directPath != nil && h.pathHasRequiredActions(directPath, context.TargetActions) {
				validPaths = append(validPaths, directPath)
				h.log.Debug().Msgf("Direct path found from subject to resource")
			}
		}
	}

	h.log.Debug().Msgf("Found %d valid paths total", len(validPaths))
	return validPaths, nil
}

// findNodesWithAssociations finds all nodes in the subgraph that have association relationships
func (h *Evaluator) findNodesWithAssociations(subgraph *Subgraph) []string {
	var nodesWithAssociations []string

	for nodeID, relationships := range subgraph.Relationships {
		for _, rel := range relationships {
			if rel.Type == model.AssociationRelationship {
				nodesWithAssociations = append(nodesWithAssociations, nodeID)
				break // Only add the node once
			}
		}
	}

	return nodesWithAssociations
}

// getAssociationsForNode gets all associations originating from a node
func (h *Evaluator) getAssociationsForNode(class string, nodeID string, subgraph *Subgraph) []*model.Association {
	var associations []*model.Association

	relationships, exists := subgraph.Relationships[nodeID]
	if !exists {
		return associations
	}

	for _, rel := range relationships {
		if rel.Type == model.AssociationRelationship {
			assoc := &model.Association{
				RelationshipID: rel.HashID,
				ClassName:      class,
			}
			// Fetch the full association details
			if err := h.datalayer.FetchAssociation(assoc, true); err == nil {
				associations = append(associations, assoc)
			}
		}
	}

	return associations
}

// associationHasRequiredActions checks if an association has the required actions
func (h *Evaluator) associationHasRequiredActions(assoc *model.Association, targetActions []string) bool {
	if len(targetActions) == 0 {
		return true // No specific actions required
	}

	if len(assoc.Verbs) == 0 {
		return false // No actions available
	}

	// Check if all required actions are present
	for _, requiredAction := range targetActions {
		found := false
		for _, verb := range assoc.Verbs {
			if verb == requiredAction {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// pathHasRequiredActions checks if a path has the required actions
func (h *Evaluator) pathHasRequiredActions(path *Path, targetActions []string) bool {
	if len(targetActions) == 0 {
		return true // No specific actions required
	}

	// Check if path has association relationships with required actions
	for _, edge := range path.Edges {
		if edge.Type == model.AssociationRelationship {
			assoc := &model.Association{
				RelationshipID: edge.HashID,
			}
			if err := h.datalayer.FetchAssociation(assoc, true); err == nil {
				if h.associationHasRequiredActions(assoc, targetActions) {
					return true
				}
			}
		}
	}

	// Check if it's a pure assignment path (which might be valid for certain use cases)
	allAssignments := true
	for _, edge := range path.Edges {
		if edge.Type != model.AssignmentRelationship {
			allAssignments = false
			break
		}
	}

	return allAssignments && len(path.Edges) > 0
}

// findPathInSubgraph finds a path between two nodes within a subgraph using BFS
func (h *Evaluator) findPathInSubgraph(class string, subgraph *Subgraph, startNodeID, endNodeID string) *Path {
	if startNodeID == endNodeID {
		// Direct path with just the start node
		if entity, exists := subgraph.Nodes[startNodeID]; exists {
			return &Path{
				Nodes:   []model.Entity{entity},
				Edges:   []model.Relationship{},
				Actions: []string{},
			}
		}
		return nil
	}

	visited := make(map[string]bool)
	parent := make(map[string]string)
	parentRel := make(map[string]*model.Relationship)
	queue := []string{startNodeID}
	visited[startNodeID] = true

	// BFS to find path
	for len(queue) > 0 {
		currentNodeID := queue[0]
		queue = queue[1:]

		if currentNodeID == endNodeID {
			// Reconstruct path
			return h.reconstructPath(class, subgraph, startNodeID, endNodeID, parent, parentRel)
		}

		// Explore neighbors
		if relationships, exists := subgraph.Relationships[currentNodeID]; exists {
			for _, rel := range relationships {
				if !visited[rel.ToID] {
					visited[rel.ToID] = true
					parent[rel.ToID] = currentNodeID
					parentRel[rel.ToID] = &rel
					queue = append(queue, rel.ToID)
				}
			}
		}
	}

	return nil // No path found
}

// reconstructPath reconstructs a path from parent pointers
func (h *Evaluator) reconstructPath(class string, subgraph *Subgraph, startNodeID, endNodeID string, parent map[string]string, parentRel map[string]*model.Relationship) *Path {
	var nodes []model.Entity
	var edges []model.Relationship
	var actions []string

	// Build path backwards
	currentNodeID := endNodeID
	pathNodeIDs := []string{currentNodeID}

	for currentNodeID != startNodeID {
		if parentNodeID, exists := parent[currentNodeID]; exists {
			pathNodeIDs = append([]string{parentNodeID}, pathNodeIDs...)
			currentNodeID = parentNodeID
		} else {
			return nil // Invalid path
		}
	}

	// Convert node IDs to entities and build edges
	for i, nodeID := range pathNodeIDs {
		if entity, exists := subgraph.Nodes[nodeID]; exists {
			nodes = append(nodes, entity)
		}

		if i > 0 {
			// Add the relationship between previous and current node
			if rel, exists := parentRel[nodeID]; exists {
				edges = append(edges, *rel)

				// If it's an association, add its actions
				if rel.Type == model.AssociationRelationship {
					assoc := &model.Association{
						RelationshipID: rel.HashID,
						ClassName:      class,
					}
					if err := h.datalayer.FetchAssociation(assoc, true); err == nil {
						actions = append(actions, assoc.Verbs...)
					}
				}
			}
		}
	}

	return &Path{
		Nodes:   nodes,
		Edges:   edges,
		Actions: actions,
	}
}

// combinePaths combines three path segments: subject->assocSource, association, intersection->resource
func (h *Evaluator) combinePaths(subjectToAssocSource *Path, assoc *model.Association, intersectionToResource *Path) *Path {
	var nodes []model.Entity
	var edges []model.Relationship
	var actions []string

	// Add subject to association source path
	nodes = append(nodes, subjectToAssocSource.Nodes...)
	edges = append(edges, subjectToAssocSource.Edges...)
	actions = append(actions, subjectToAssocSource.Actions...)

	// Add the association relationship
	if assoc.Relationship != nil {
		edges = append(edges, *assoc.Relationship)
		actions = append(actions, assoc.Verbs...)
	}

	// Add intersection to resource path (skip first node to avoid duplication)
	if len(intersectionToResource.Nodes) > 1 {
		nodes = append(nodes, intersectionToResource.Nodes[1:]...)
	}
	edges = append(edges, intersectionToResource.Edges...)
	actions = append(actions, intersectionToResource.Actions...)

	return &Path{
		Nodes:   nodes,
		Edges:   edges,
		Actions: actions,
	}
}

// checkProhibitionsForRequest checks if any prohibitions apply to the request
func (h *Evaluator) checkProhibitionsForRequest(_ context.Context, req *Request, privilegePaths []*Path) ([]*model.Prohibition, error) {
	h.log.Debug().Msgf("Checking prohibitions for subject %s on resource %s", req.Subject.EntityID, req.Resource.EntityID)

	var applicableProhibitions []*model.Prohibition

	// Get all prohibitions for the policy class
	prohibitions, err := h.datalayer.FetchProhibitionsForPolicyClass(req.PolicyClass)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prohibitions for policy class %s: %w", req.PolicyClass, err)
	}

	h.log.Debug().Msgf("Found %d prohibitions for policy class %s", len(prohibitions), req.PolicyClass)

	// Check each prohibition to see if it applies to this request
	for _, prohibition := range prohibitions {
		if h.prohibitionApplies(prohibition, req, privilegePaths) {
			applicableProhibitions = append(applicableProhibitions, prohibition)
			h.log.Debug().Msgf("Prohibition %s applies to request", prohibition.RelationshipID)
		}
	}

	return applicableProhibitions, nil
}

// prohibitionApplies checks if a prohibition applies to the current request
func (h *Evaluator) prohibitionApplies(prohibition *model.Prohibition, req *Request, privilegePaths []*Path) bool {
	// Check if prohibition applies to any of the requested actions
	for _, action := range req.Actions {
		if prohibition.HasOperation(action) {
			// Check if the prohibition's relationship intersects with any privilege paths
			if h.prohibitionIntersectsWithPaths(prohibition, req, privilegePaths) {
				return true
			}
		}
	}
	return false
}

// prohibitionIntersectsWithPaths checks if a prohibition intersects with any of the privilege paths
func (h *Evaluator) prohibitionIntersectsWithPaths(prohibition *model.Prohibition, req *Request, privilegePaths []*Path) bool {
	if prohibition.Relationship == nil {
		return false
	}

	// Check if the prohibition applies to the subject or any entity in the privilege paths
	for _, path := range privilegePaths {
		// Check if prohibition applies to the subject
		if prohibition.Relationship.FromID == req.Subject.EntityID {
			return true
		}

		// Check if prohibition applies to any node in the path
		for _, node := range path.Nodes {
			if prohibition.Relationship.FromID == node.HashID {
				return true
			}
		}

		// Check if prohibition denies access to the resource or any attribute containing it
		if prohibition.Relationship.ToID == req.Resource.EntityID {
			return true
		}

		// Check if prohibition applies to any target in the path
		for _, node := range path.Nodes {
			if prohibition.Relationship.ToID == node.HashID {
				return true
			}
		}
	}

	return false
}

// extractObligations extracts obligations from privilege paths
func (h *Evaluator) extractObligations(privilegePaths []*Path) []string {
	obligationSet := make(map[string]struct{})
	var obligations []string

	h.log.Debug().Msgf("Extracting obligations from %d privilege paths", len(privilegePaths))

	for _, path := range privilegePaths {
		// Extract obligations from association relationships in the path
		for _, edge := range path.Edges {
			if edge.Type == model.AssociationRelationship {
				assoc := &model.Association{
					RelationshipID: edge.HashID,
				}
				if err := h.datalayer.FetchAssociation(assoc, true); err == nil {
					// Add obligations from this association
					for _, obligation := range assoc.Obligations {
						if _, exists := obligationSet[obligation]; !exists {
							obligationSet[obligation] = struct{}{}
							obligations = append(obligations, obligation)
						}
					}
				}
			}
		}
	}

	h.log.Debug().Msgf("Extracted %d unique obligations", len(obligations))
	return obligations
}

// extractPolicyPath extracts the policy path from privilege paths for audit/debugging
func (h *Evaluator) extractPolicyPath(privilegePaths []*Path) []*model.Entity {
	if len(privilegePaths) == 0 {
		return nil
	}

	// Return the nodes from the first privilege path for simplicity
	// In a more sophisticated implementation, you might want to merge or select the "best" path
	firstPath := privilegePaths[0]
	var policyPath []*model.Entity

	for _, node := range firstPath.Nodes {
		nodeCopy := node // Create a copy to avoid pointer issues
		policyPath = append(policyPath, &nodeCopy)
	}

	h.log.Debug().Msgf("Extracted policy path with %d entities", len(policyPath))
	return policyPath
}

// // Legacy function - kept for backward compatibility but replaced by subgraph approach
// // TODO: Remove this function after migration is complete
// func (h *Evaluator) getPaths(class string, subjectEntity model.Entity, resourceEntity model.Entity, targetActions []string) ([]*Path, error) {
// 	h.log.Debug().Msgf("Using legacy path evaluation - consider migrating to subgraph approach")
// 	return h.evaluatePathsUsingSubgraphs(class, subjectEntity, resourceEntity, targetActions)
// }
