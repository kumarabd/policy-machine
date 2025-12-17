package engine

import "github.com/RoaringBitmap/roaring"

// Returns closure of a UA node: itself + all ancestors via uaParents.
func (e *Engine) uaNodeAllParents(s *Snapshot, ua uint32) *roaring.Bitmap {
	if b, ok := e.uaNodeClosure.Get(ua); ok {
		return b
	}

	out := roaring.New()
	queue := []uint32{ua}
	out.Add(ua)

	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		for _, p := range s.uaParents[cur] {
			if out.CheckedAdd(p) {
				queue = append(queue, p)
			}
		}
	}

	e.uaNodeClosure.Put(ua, out)
	return out
}

func (e *Engine) oaNodeAllParents(s *Snapshot, oa uint32) *roaring.Bitmap {
	if b, ok := e.oaNodeClosure.Get(oa); ok {
		return b
	}

	out := roaring.New()
	queue := []uint32{oa}
	out.Add(oa)

	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		for _, p := range s.oaParents[cur] {
			if out.CheckedAdd(p) {
				queue = append(queue, p)
			}
		}
	}

	e.oaNodeClosure.Put(oa, out)
	return out
}
