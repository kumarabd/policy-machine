package engine

import (
	"errors"

	"github.com/RoaringBitmap/roaring"
)

var ErrTraversalLimitExceeded = errors.New("traversal limit exceeded: graph too large")

// Returns closure of a UA node: itself + all ancestors via uaParents.
func (e *Engine) uaNodeAllParents(s *Snapshot, ua uint32) *roaring.Bitmap {
	if b, ok := e.caches.UANodeClosure.Get(ua); ok {
		return b
	}

	out := roaring.New()
	queue := []uint32{ua}
	out.Add(ua)

	for i := 0; i < len(queue); i++ {
		if out.GetCardinality() > uint64(e.maxTraversalNodes) {
			// Safety: deny access if traversal limit exceeded
			return roaring.New() // Return empty bitmap (deny)
		}
		cur := queue[i]
		for _, p := range s.uaParents[cur] {
			if out.CheckedAdd(p) {
				queue = append(queue, p)
			}
		}
	}

	e.caches.UANodeClosure.Put(ua, out)
	return out
}

func (e *Engine) oaNodeAllParents(s *Snapshot, oa uint32) *roaring.Bitmap {
	if b, ok := e.caches.OANodeClosure.Get(oa); ok {
		return b
	}

	out := roaring.New()
	queue := []uint32{oa}
	out.Add(oa)

	for i := 0; i < len(queue); i++ {
		if out.GetCardinality() > uint64(e.maxTraversalNodes) {
			// Safety: deny access if traversal limit exceeded
			return roaring.New() // Return empty bitmap (deny)
		}
		cur := queue[i]
		for _, p := range s.oaParents[cur] {
			if out.CheckedAdd(p) {
				queue = append(queue, p)
			}
		}
	}

	e.caches.OANodeClosure.Put(oa, out)
	return out
}
