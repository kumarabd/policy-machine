package engine

import (
	"github.com/RoaringBitmap/roaring"
)

func commonPCs(s *Snapshot, uaClosure, oaClosure *roaring.Bitmap) *roaring.Bitmap {
	userPCs := roaring.New()
	it := uaClosure.Iterator()
	for it.HasNext() {
		ua := it.Next()
		if b := s.uaToPCs[ua]; b != nil {
			userPCs.Or(b)
		}
	}

	objPCs := roaring.New()
	it2 := oaClosure.Iterator()
	for it2.HasNext() {
		oa := it2.Next()
		if b := s.oaToPCs[oa]; b != nil {
			objPCs.Or(b)
		}
	}

	userPCs.And(objPCs)
	return userPCs // intersection
}
