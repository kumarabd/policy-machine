package engine

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
)

// userUAClosure returns bitmap of UA indices reachable from user's assigned UAs (including them)
func (e *Engine) userUAClosure(s *Snapshot, userID uuid.UUID) *roaring.Bitmap {
	if bmp, ok := e.uaCache.Get(userID); ok {
		return bmp
	}

	out := roaring.New()
	for _, ua := range s.userToUAs[userID] {
		out.Or(e.uaNodeAllParents(s, ua))
	}

	e.uaCache.Put(userID, out)
	return out
}

// objectOAClosure returns bitmap of OA indices reachable from object's assigned OAs (including them)
func (e *Engine) objectOAClosure(s *Snapshot, objectID uuid.UUID) *roaring.Bitmap {
	if bmp, ok := e.oaCache.Get(objectID); ok {
		return bmp
	}

	out := roaring.New()
	for _, oa := range s.objectToOAs[objectID] {
		out.Or(e.oaNodeAllParents(s, oa))
	}

	e.oaCache.Put(objectID, out)
	return out
}
