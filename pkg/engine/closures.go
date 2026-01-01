package engine

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
)

// SubjectUAClosure returns bitmap of UA indices reachable from subject's assigned UAs (including them) (public for explain)
func (e *Engine) SubjectUAClosure(s *Snapshot, subjectID uuid.UUID) *roaring.Bitmap {
	return e.subjectUAClosure(s, subjectID)
}

// subjectUAClosure returns bitmap of UA indices reachable from subject's assigned UAs (including them)
func (e *Engine) subjectUAClosure(s *Snapshot, subjectID uuid.UUID) *roaring.Bitmap {
	if bmp, ok := e.caches.UACache.Get(subjectID); ok {
		return bmp
	}

	out := roaring.New()
	for _, ua := range s.subjectToUAs[subjectID] {
		out.Or(e.uaNodeAllParents(s, ua))
	}

	e.caches.UACache.Put(subjectID, out)
	return out
}

// ObjectOAClosure returns bitmap of OA indices reachable from object's assigned OAs (including them) (public for explain)
func (e *Engine) ObjectOAClosure(s *Snapshot, objectID uuid.UUID) *roaring.Bitmap {
	return e.objectOAClosure(s, objectID)
}

// objectOAClosure returns bitmap of OA indices reachable from object's assigned OAs (including them)
func (e *Engine) objectOAClosure(s *Snapshot, objectID uuid.UUID) *roaring.Bitmap {
	if bmp, ok := e.caches.OACache.Get(objectID); ok {
		return bmp
	}

	out := roaring.New()
	for _, oa := range s.objectToOAs[objectID] {
		out.Or(e.oaNodeAllParents(s, oa))
	}

	e.caches.OACache.Put(objectID, out)
	return out
}
