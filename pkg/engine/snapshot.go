package engine

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
)

type Snapshot struct {
	TenantID uuid.UUID
	Version  int64 // revision
	LastSeq  int64 // last applied outbox seq

	// UA indexing (uuid -> dense int)
	uaIndex map[uuid.UUID]uint32
	uaByIdx []uuid.UUID

	// OA indexing (uuid -> dense int)
	oaIndex map[uuid.UUID]uint32
	oaByIdx []uuid.UUID

	// PC indexing (uuid -> dense int)
	pcIndex map[uuid.UUID]uint32
	pcByIdx []uuid.UUID
	uaToPCs map[uint32]*roaring.Bitmap // UA idx -> PCs
	oaToPCs map[uint32]*roaring.Bitmap // OA idx -> PCs

	// Assignment graphs
	userToUAs     map[uuid.UUID][]uint32 // userID -> uaIdx children
	uaParents     map[uint32][]uint32    // uaIdx -> parent uaIdxs
	uaChildren    map[uint32][]uint32    // reverse uaParents
	uaDirectUsers map[uint32][]uuid.UUID // reverse userToUAs

	objectToOAs     map[uuid.UUID][]uint32 // objectID -> oaIdx children
	oaParents       map[uint32][]uint32    // oaIdx -> parent oaIdxs
	oaChildren      map[uint32][]uint32    // reverse oaParents
	oaDirectObjects map[uint32][]uuid.UUID // reverse objectToOAs

	// Associations: uaIdx -> op -> bitmap(OA indices)
	assoc map[uint32]map[string]*roaring.Bitmap

	// Prohibitions:
	// - userProhibits[userID][op] => bitmap(OA indices)
	// - uaProhibits[uaIdx][op] => bitmap(OA indices)
	userProhibits map[uuid.UUID]map[string]*roaring.Bitmap
	uaProhibits   map[uint32]map[string]*roaring.Bitmap
}

// UAByIdx returns the UA ID array (for explain endpoint)
func (s *Snapshot) UAByIdx() []uuid.UUID {
	return s.uaByIdx
}

// OAByIdx returns the OA ID array (for explain endpoint)
func (s *Snapshot) OAByIdx() []uuid.UUID {
	return s.oaByIdx
}

func (s *Snapshot) UsersInUASubtree(root uint32) []uuid.UUID {
	seenUA := map[uint32]struct{}{}
	seenUser := map[uuid.UUID]struct{}{}

	queue := []uint32{root}
	seenUA[root] = struct{}{}

	const maxTraversalNodes = 100000 // Safety limit
	for i := 0; i < len(queue); i++ {
		if len(seenUA) > maxTraversalNodes {
			// Safety: stop traversal if limit exceeded
			break
		}
		ua := queue[i]
		for _, u := range s.uaDirectUsers[ua] {
			seenUser[u] = struct{}{}
		}
		for _, child := range s.uaChildren[ua] {
			if _, ok := seenUA[child]; ok {
				continue
			}
			seenUA[child] = struct{}{}
			queue = append(queue, child)
		}
	}
	out := make([]uuid.UUID, 0, len(seenUser))
	for u := range seenUser {
		out = append(out, u)
	}
	return out
}

func (s *Snapshot) ObjectsInOASubtree(root uint32) []uuid.UUID {
	seenOA := map[uint32]struct{}{}
	seenObj := map[uuid.UUID]struct{}{}

	queue := []uint32{root}
	seenOA[root] = struct{}{}

	const maxTraversalNodes = 100000 // Safety limit
	for i := 0; i < len(queue); i++ {
		if len(seenOA) > maxTraversalNodes {
			// Safety: stop traversal if limit exceeded
			break
		}
		oa := queue[i]
		for _, o := range s.oaDirectObjects[oa] {
			seenObj[o] = struct{}{}
		}
		for _, child := range s.oaChildren[oa] {
			if _, ok := seenOA[child]; ok {
				continue
			}
			seenOA[child] = struct{}{}
			queue = append(queue, child)
		}
	}
	out := make([]uuid.UUID, 0, len(seenObj))
	for o := range seenObj {
		out = append(out, o)
	}
	return out
}
