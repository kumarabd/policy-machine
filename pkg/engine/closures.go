package engine

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
)

// UserUAClosure returns bitmap of UA indices reachable from user's assigned UAs (including them) (public for explain)
func (e *Engine) UserUAClosure(s *Snapshot, userID uuid.UUID) *roaring.Bitmap {
	return e.userUAClosure(s, userID)
}

// userUAClosure returns bitmap of UA indices reachable from user's assigned UAs (including them)
func (e *Engine) userUAClosure(s *Snapshot, userID uuid.UUID) *roaring.Bitmap {
	if bmp, ok := e.caches.UACache.Get(userID); ok {
		return bmp
	}

	out := roaring.New()
	for _, ua := range s.userToUAs[userID] {
		out.Or(e.uaNodeAllParents(s, ua))
	}

	e.caches.UACache.Put(userID, out)
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
