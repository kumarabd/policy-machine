package engine

import (
	"encoding/json"
	"fmt"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/postgres"
)

func applyChange(s *Snapshot, ch postgres.PolicyChange) error {
	switch ch.Kind {

	case "ASSIGNMENT_EDGE":
		var p struct {
			ChildType  postgres.NodeType `json:"child_type"`
			ChildID    uuid.UUID         `json:"child_id"`
			ParentType postgres.NodeType `json:"parent_type"`
			ParentID   uuid.UUID         `json:"parent_id"`
		}
		if err := json.Unmarshal(ch.Payload, &p); err != nil {
			return err
		}
		return applyAssignmentEdge(s, ch.Op, p.ChildType, p.ChildID, p.ParentType, p.ParentID)

	case "ASSOC_OP":
		var p struct {
			SubjectType string    `json:"subject_type"`
			SubjectID   uuid.UUID `json:"subject_id"`
			ObjectType  string    `json:"object_type"`
			ObjectID    uuid.UUID `json:"object_id"`
			Operation   string    `json:"op"`
		}
		if err := json.Unmarshal(ch.Payload, &p); err != nil {
			return err
		}
		// Engine currently only supports UA->OA associations
		// Note: "subject-set" and "object-set" are API terminology only for the subject-sets/object-sets endpoints.
		// In rules API, we use "subject-attribute" and "object-attribute" to refer to the actual entities.
		if p.SubjectType != "subject-attribute" || p.ObjectType != "object-attribute" {
			return nil // Skip non-UA->OA associations
		}
		return applyAssocOp(s, ch.Op, p.SubjectID, p.ObjectID, p.Operation)

	case "PROHIB_OP":
		var p struct {
			SubjectType       postgres.ProhibitionSubjectType `json:"subject_type"`
			SubjectID         uuid.UUID                       `json:"subject_id"`
			ObjectAttributeID uuid.UUID                       `json:"oa_id"`
			Operation         string                          `json:"op"`
		}
		if err := json.Unmarshal(ch.Payload, &p); err != nil {
			return err
		}
		return applyProhibOp(s, ch.Op, p.SubjectType, p.SubjectID, p.ObjectAttributeID, p.Operation)

	default:
		// Unknown change kind -> safer to full rebuild.
		return fmt.Errorf("unknown change kind: %s", ch.Kind)
	}
}

func ensureUAIdx(s *Snapshot, uaID uuid.UUID) (uint32, bool) {
	if idx, ok := s.uaIndex[uaID]; ok {
		return idx, true
	}
	// Allow append-only expansion (no compaction)
	idx := uint32(len(s.uaByIdx))
	s.uaByIdx = append(s.uaByIdx, uaID)
	s.uaIndex[uaID] = idx
	return idx, true
}

func ensureOAIdx(s *Snapshot, oaID uuid.UUID) (uint32, bool) {
	if idx, ok := s.oaIndex[oaID]; ok {
		return idx, true
	}
	idx := uint32(len(s.oaByIdx))
	s.oaByIdx = append(s.oaByIdx, oaID)
	s.oaIndex[oaID] = idx
	return idx, true
}
func applyAssignmentEdge(s *Snapshot, op postgres.ChangeOp,
	childType postgres.NodeType, childID uuid.UUID,
	parentType postgres.NodeType, parentID uuid.UUID) error {

	switch {

	// -------- SUBJECT -> UA --------
	case childType == postgres.NodeSubject && parentType == postgres.NodeUA:
		uaIdx, _ := ensureUAIdx(s, parentID)

		// copy-on-write maps we mutate
		s.subjectToUAs = cowMapSlice(s.subjectToUAs)
		s.uaDirectSubjects = cowMapSliceUUID(s.uaDirectSubjects)

		if op == postgres.OpAdd {
			s.subjectToUAs[childID] = addU32Unique(s.subjectToUAs[childID], uaIdx)
			s.uaDirectSubjects[uaIdx] = addUUIDUnique(s.uaDirectSubjects[uaIdx], childID)
		} else if op == postgres.OpRemove {
			s.subjectToUAs[childID] = removeU32(s.subjectToUAs[childID], uaIdx)
			s.uaDirectSubjects[uaIdx] = removeUUID(s.uaDirectSubjects[uaIdx], childID)
		}
		return nil

	// -------- UA -> UA (child -> parent) --------
	case childType == postgres.NodeUA && parentType == postgres.NodeUA:
		childIdx, _ := ensureUAIdx(s, childID)
		parentIdx, _ := ensureUAIdx(s, parentID)

		s.uaParents = cowMapSliceU32(s.uaParents)
		s.uaChildren = cowMapSliceU32(s.uaChildren)

		if op == postgres.OpAdd {
			s.uaParents[childIdx] = addU32Unique(s.uaParents[childIdx], parentIdx)
			s.uaChildren[parentIdx] = addU32Unique(s.uaChildren[parentIdx], childIdx)
		} else if op == postgres.OpRemove {
			s.uaParents[childIdx] = removeU32(s.uaParents[childIdx], parentIdx)
			s.uaChildren[parentIdx] = removeU32(s.uaChildren[parentIdx], childIdx)
		}
		return nil

	// -------- OBJECT -> OA --------
	case childType == postgres.NodeObject && parentType == postgres.NodeOA:
		oaIdx, _ := ensureOAIdx(s, parentID)

		s.objectToOAs = cowMapSlice(s.objectToOAs)
		s.oaDirectObjects = cowMapSliceUUID(s.oaDirectObjects)

		if op == postgres.OpAdd {
			s.objectToOAs[childID] = addU32Unique(s.objectToOAs[childID], oaIdx)
			s.oaDirectObjects[oaIdx] = addUUIDUnique(s.oaDirectObjects[oaIdx], childID)
		} else if op == postgres.OpRemove {
			s.objectToOAs[childID] = removeU32(s.objectToOAs[childID], oaIdx)
			s.oaDirectObjects[oaIdx] = removeUUID(s.oaDirectObjects[oaIdx], childID)
		}
		return nil

	// -------- OA -> OA (child -> parent) --------
	case childType == postgres.NodeOA && parentType == postgres.NodeOA:
		childIdx, _ := ensureOAIdx(s, childID)
		parentIdx, _ := ensureOAIdx(s, parentID)

		s.oaParents = cowMapSliceU32(s.oaParents)
		s.oaChildren = cowMapSliceU32(s.oaChildren)

		if op == postgres.OpAdd {
			s.oaParents[childIdx] = addU32Unique(s.oaParents[childIdx], parentIdx)
			s.oaChildren[parentIdx] = addU32Unique(s.oaChildren[parentIdx], childIdx)
		} else if op == postgres.OpRemove {
			s.oaParents[childIdx] = removeU32(s.oaParents[childIdx], parentIdx)
			s.oaChildren[parentIdx] = removeU32(s.oaChildren[parentIdx], childIdx)
		}
		return nil
	case childType == postgres.NodeUA && parentType == postgres.NodePolicyClass:
		uaIdx, _ := ensureUAIdx(s, childID)
		pcIdx := ensurePCIdx(s, parentID)

		// copy-on-write map
		if s.uaToPCs == nil {
			s.uaToPCs = make(map[uint32]*roaring.Bitmap)
		} else {
			s.uaToPCs = cowMapBitmapU32(s.uaToPCs)
		}

		b := s.uaToPCs[uaIdx]
		if b == nil {
			b = roaring.New()
		} else {
			b = b.Clone()
		}

		if op == postgres.OpAdd {
			b.Add(pcIdx)
		} else if op == postgres.OpRemove {
			b.Remove(pcIdx)
		}
		s.uaToPCs[uaIdx] = b
		return nil

	case childType == postgres.NodeOA && parentType == postgres.NodePolicyClass:
		oaIdx, _ := ensureOAIdx(s, childID)
		pcIdx := ensurePCIdx(s, parentID)

		if s.oaToPCs == nil {
			s.oaToPCs = make(map[uint32]*roaring.Bitmap)
		} else {
			s.oaToPCs = cowMapBitmapU32(s.oaToPCs)
		}

		b := s.oaToPCs[oaIdx]
		if b == nil {
			b = roaring.New()
		} else {
			b = b.Clone()
		}

		if op == postgres.OpAdd {
			b.Add(pcIdx)
		} else if op == postgres.OpRemove {
			b.Remove(pcIdx)
		}
		s.oaToPCs[oaIdx] = b
		return nil
	}

	return nil
}

func applyAssocOp(s *Snapshot, op postgres.ChangeOp, uaID, oaID uuid.UUID, operation string) error {
	uaIdx, _ := ensureUAIdx(s, uaID)
	oaIdx, _ := ensureOAIdx(s, oaID)

	// Copy-on-write maps
	s.assoc = cowMapAssoc(s.assoc)
	if s.assoc[uaIdx] == nil {
		s.assoc[uaIdx] = make(map[string]*roaring.Bitmap)
	}
	// Clone bitmap on write
	b := s.assoc[uaIdx][operation]
	if b == nil {
		b = roaring.New()
	} else {
		b = b.Clone()
	}
	if op == postgres.OpAdd {
		b.Add(oaIdx)
	} else if op == postgres.OpRemove {
		b.Remove(oaIdx)
	}
	s.assoc[uaIdx][operation] = b
	return nil
}

func applyProhibOp(s *Snapshot, op postgres.ChangeOp, subjectType postgres.ProhibitionSubjectType, subjectID uuid.UUID, oaID uuid.UUID, operation string) error {
	oaIdx, _ := ensureOAIdx(s, oaID)

	if subjectType == postgres.ProhibitSubject {
		s.subjectProhibits = cowMapSubjectOps(s.subjectProhibits)
		if s.subjectProhibits[subjectID] == nil {
			s.subjectProhibits[subjectID] = make(map[string]*roaring.Bitmap)
		}
		b := s.subjectProhibits[subjectID][operation]
		if b == nil {
			b = roaring.New()
		} else {
			b = b.Clone()
		}
		if op == postgres.OpAdd {
			b.Add(oaIdx)
		} else if op == postgres.OpRemove {
			b.Remove(oaIdx)
		}
		s.subjectProhibits[subjectID][operation] = b
		return nil
	}

	// UA prohibition
	uaIdx, _ := ensureUAIdx(s, subjectID)
	s.uaProhibits = cowMapUAOps(s.uaProhibits)
	if s.uaProhibits[uaIdx] == nil {
		s.uaProhibits[uaIdx] = make(map[string]*roaring.Bitmap)
	}
	b := s.uaProhibits[uaIdx][operation]
	if b == nil {
		b = roaring.New()
	} else {
		b = b.Clone()
	}
	if op == postgres.OpAdd {
		b.Add(oaIdx)
	} else if op == postgres.OpRemove {
		b.Remove(oaIdx)
	}
	s.uaProhibits[uaIdx][operation] = b
	return nil
}

// --- small helpers ---

func removeUint32(in []uint32, x uint32) []uint32 {
	out := in[:0]
	for _, v := range in {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func cowMapSlice[M ~map[uuid.UUID][]uint32](m M) M {
	cp := make(M, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
func cowMapSliceU32(m map[uint32][]uint32) map[uint32][]uint32 {
	cp := make(map[uint32][]uint32, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
func cowMapAssoc(m map[uint32]map[string]*roaring.Bitmap) map[uint32]map[string]*roaring.Bitmap {
	cp := make(map[uint32]map[string]*roaring.Bitmap, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
func cowMapSubjectOps(m map[uuid.UUID]map[string]*roaring.Bitmap) map[uuid.UUID]map[string]*roaring.Bitmap {
	cp := make(map[uuid.UUID]map[string]*roaring.Bitmap, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
func cowMapUAOps(m map[uint32]map[string]*roaring.Bitmap) map[uint32]map[string]*roaring.Bitmap {
	cp := make(map[uint32]map[string]*roaring.Bitmap, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func addUUIDUnique(in []uuid.UUID, x uuid.UUID) []uuid.UUID {
	for _, v := range in {
		if v == x {
			return in
		}
	}
	return append(in, x)
}
func removeUUID(in []uuid.UUID, x uuid.UUID) []uuid.UUID {
	out := in[:0]
	for _, v := range in {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func addU32Unique(in []uint32, x uint32) []uint32 {
	for _, v := range in {
		if v == x {
			return in
		}
	}
	return append(in, x)
}
func removeU32(in []uint32, x uint32) []uint32 {
	out := in[:0]
	for _, v := range in {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func cowMapSliceUUID(m map[uint32][]uuid.UUID) map[uint32][]uuid.UUID {
	cp := make(map[uint32][]uuid.UUID, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func cowMapBitmapU32(m map[uint32]*roaring.Bitmap) map[uint32]*roaring.Bitmap {
	cp := make(map[uint32]*roaring.Bitmap, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
