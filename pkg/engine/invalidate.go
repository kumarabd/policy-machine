package engine

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/postgres"
)

type invalidation struct {
	subjectsUAClosure map[uuid.UUID]struct{} // invalidate UA closure (+ allow/deny all ops)
	objectsOAClosure  map[uuid.UUID]struct{} // invalidate OA closure

	subjectAllowOp map[uuid.UUID]map[string]struct{}
	subjectDenyOp  map[uuid.UUID]map[string]struct{}

	uaNodeClosures map[uint32]struct{}
	oaNodeClosures map[uint32]struct{}

	// decisions-only invalidations
	subjectsDecisionsOnly map[uuid.UUID]struct{}
	objectsDecisionsOnly  map[uuid.UUID]struct{}
	oaDescClosures        map[uint32]struct{}
}

func newInvalidation() *invalidation {
	return &invalidation{
		subjectsUAClosure:     make(map[uuid.UUID]struct{}),
		objectsOAClosure:      make(map[uuid.UUID]struct{}),
		subjectAllowOp:        make(map[uuid.UUID]map[string]struct{}),
		subjectDenyOp:         make(map[uuid.UUID]map[string]struct{}),
		uaNodeClosures:        make(map[uint32]struct{}),
		oaNodeClosures:        make(map[uint32]struct{}),
		subjectsDecisionsOnly: make(map[uuid.UUID]struct{}),
		objectsDecisionsOnly:  make(map[uuid.UUID]struct{}),
		oaDescClosures:        make(map[uint32]struct{}),
	}
}

func (inv *invalidation) addAllow(subject uuid.UUID, op string) {
	if inv.subjectAllowOp[subject] == nil {
		inv.subjectAllowOp[subject] = map[string]struct{}{}
	}
	inv.subjectAllowOp[subject][op] = struct{}{}
}
func (inv *invalidation) addDeny(subject uuid.UUID, op string) {
	if inv.subjectDenyOp[subject] == nil {
		inv.subjectDenyOp[subject] = map[string]struct{}{}
	}
	inv.subjectDenyOp[subject][op] = struct{}{}
}

func computeInvalidations(inv *invalidation, s *Snapshot, ch postgres.PolicyChange) {
	switch ch.Kind {

	case "ASSIGNMENT_EDGE":
		var p struct {
			ChildType  postgres.NodeType `json:"child_type"`
			ChildID    uuid.UUID         `json:"child_id"`
			ParentType postgres.NodeType `json:"parent_type"`
			ParentID   uuid.UUID         `json:"parent_id"`
		}
		if json.Unmarshal(ch.Payload, &p) != nil {
			return
		}

		// subject -> UA : only that subject’s UA closure changes
		if p.ChildType == postgres.NodeSubject && p.ParentType == postgres.NodeUA {
			inv.subjectsUAClosure[p.ChildID] = struct{}{}
			return
		}

		// UA -> UA : all subjects assigned under CHILD UA subtree affected
		if p.ChildType == postgres.NodeUA && p.ParentType == postgres.NodeUA {
			childIdx, ok := s.uaIndex[p.ChildID]
			if !ok {
				return
			}
			// invalidate UA node closures for every UA in subtree(childIdx)
			for _, ua := range s.UASubtree(childIdx) {
				inv.uaNodeClosures[ua] = struct{}{}
			}

			// plus: impacted subjects (you already do this)
			for _, u := range s.SubjectsInUASubtree(childIdx) {
				inv.subjectsUAClosure[u] = struct{}{}
			}
			return
		}

		// object -> OA : only that object’s OA closure changes
		if p.ChildType == postgres.NodeObject && p.ParentType == postgres.NodeOA {
			inv.objectsOAClosure[p.ChildID] = struct{}{}
			return
		}

		// OA -> OA : all objects assigned under CHILD OA subtree affected
		if p.ChildType == postgres.NodeOA && p.ParentType == postgres.NodeOA {
			parentIdx, ok := s.oaIndex[p.ParentID]
			if !ok {
				return
			}
			addOAAncestors(s, parentIdx, inv.oaDescClosures)
			childIdx, ok := s.oaIndex[p.ChildID]
			if !ok {
				return
			}

			// invalidate UA node closures for every UA in subtree(childIdx)
			for _, oa := range s.OASubtree(childIdx) { // implement similarly to SubjectsInUASubtree but returns []uint32
				inv.oaNodeClosures[oa] = struct{}{}
			}

			// plus: impacted subjects (you already do this)
			for _, o := range s.ObjectsInOASubtree(childIdx) {
				inv.objectsOAClosure[o] = struct{}{}
			}
			return
		}

		// UA -> PC
		if p.ChildType == postgres.NodeUA && p.ParentType == postgres.NodePolicyClass {
			uaIdx, ok := s.uaIndex[p.ChildID]
			if !ok {
				return
			}
			for _, u := range s.SubjectsInUASubtree(uaIdx) {
				inv.subjectsDecisionsOnly[u] = struct{}{}
			}
			return
		}

		// OA -> PC
		if p.ChildType == postgres.NodeOA && p.ParentType == postgres.NodePolicyClass {
			oaIdx, ok := s.oaIndex[p.ChildID]
			if !ok {
				return
			}
			for _, o := range s.ObjectsInOASubtree(oaIdx) {
				inv.objectsDecisionsOnly[o] = struct{}{}
			}
			return
		}

	case "ASSOC_OP":
		// association changes do NOT affect closures, but affect allowCache(subject,op)
		var p struct {
			SubjectType string    `json:"subject_type"`
			SubjectID   uuid.UUID `json:"subject_id"`
			ObjectType  string    `json:"object_type"`
			ObjectID    uuid.UUID `json:"object_id"`
			Operation   string    `json:"op"`
		}
		if json.Unmarshal(ch.Payload, &p) != nil {
			return
		}
		// Engine currently only supports UA->OA associations
		// Note: "subject-set" and "object-set" are API terminology only for the subject-sets/object-sets endpoints.
		// In rules API, we use "subject-attribute" and "object-attribute" to refer to the actual entities.
		if p.SubjectType != "subject-attribute" || p.ObjectType != "object-attribute" {
			return // Skip non-UA->OA associations
		}

		uaIdx, ok := s.uaIndex[p.SubjectID]
		if !ok {
			return
		}

		for _, u := range s.SubjectsInUASubtree(uaIdx) {
			inv.addAllow(u, p.Operation)
		}

	case "PROHIB_OP":
		// prohibition changes do NOT affect closures, but affect denyCache(subject,op)
		var p struct {
			SubjectType       postgres.ProhibitionSubjectType `json:"subject_type"`
			SubjectID         uuid.UUID                       `json:"subject_id"`
			ObjectAttributeID uuid.UUID                       `json:"oa_id"`
			Operation         string                          `json:"op"`
		}
		if json.Unmarshal(ch.Payload, &p) != nil {
			return
		}

		if p.SubjectType == postgres.ProhibitSubject {
			inv.addDeny(p.SubjectID, p.Operation)
			return
		}

		uaIdx, ok := s.uaIndex[p.SubjectID]
		if !ok {
			return
		}
		for _, u := range s.SubjectsInUASubtree(uaIdx) {
			inv.addDeny(u, p.Operation)
		}
	}
}

// Helpers
func (s *Snapshot) UASubtree(root uint32) []uint32 {
	seen := map[uint32]struct{}{root: {}}
	q := []uint32{root}
	const maxTraversalNodes = 100000 // Safety limit
	for i := 0; i < len(q); i++ {
		if len(seen) > maxTraversalNodes {
			// Safety: stop traversal if limit exceeded
			break
		}
		cur := q[i]
		for _, child := range s.uaChildren[cur] {
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			q = append(q, child)
		}
	}
	out := make([]uint32, 0, len(seen))
	for ua := range seen {
		out = append(out, ua)
	}
	return out
}

func (s *Snapshot) OASubtree(root uint32) []uint32 {
	seen := map[uint32]struct{}{root: {}}
	q := []uint32{root}
	const maxTraversalNodes = 100000 // Safety limit
	for i := 0; i < len(q); i++ {
		if len(seen) > maxTraversalNodes {
			// Safety: stop traversal if limit exceeded
			break
		}
		cur := q[i]
		for _, child := range s.oaChildren[cur] {
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			q = append(q, child)
		}
	}
	out := make([]uint32, 0, len(seen))
	for ua := range seen {
		out = append(out, ua)
	}
	return out
}

func ensurePCIdx(s *Snapshot, pcID uuid.UUID) uint32 {
	if idx, ok := s.pcIndex[pcID]; ok {
		return idx
	}
	idx := uint32(len(s.pcByIdx))
	s.pcByIdx = append(s.pcByIdx, pcID)
	s.pcIndex[pcID] = idx
	return idx
}

func addOAAncestors(s *Snapshot, start uint32, out map[uint32]struct{}) {
	queue := []uint32{start}
	seen := map[uint32]struct{}{start: {}}

	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		out[cur] = struct{}{}
		for _, p := range s.oaParents[cur] {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			queue = append(queue, p)
		}
	}
}
