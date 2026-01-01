package engine

import (
	"context"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"gorm.io/gorm"
)

func LoadSnapshot(ctx context.Context, db *gorm.DB, tenantID string) (*Snapshot, error) {
	s := &Snapshot{
		TenantID: tenantID,

		uaIndex: make(map[uuid.UUID]uint32),
		uaByIdx: make([]uuid.UUID, 0),
		oaIndex: make(map[uuid.UUID]uint32),
		oaByIdx: make([]uuid.UUID, 0),

		subjectToUAs:     make(map[uuid.UUID][]uint32),
		uaParents:        make(map[uint32][]uint32),
		uaChildren:       make(map[uint32][]uint32),
		uaDirectSubjects: make(map[uint32][]uuid.UUID),
		objectToOAs:      make(map[uuid.UUID][]uint32),
		oaParents:        make(map[uint32][]uint32),
		oaChildren:       make(map[uint32][]uint32),
		oaDirectObjects:  make(map[uint32][]uuid.UUID),

		assoc:            make(map[uint32]map[string]*roaring.Bitmap),
		subjectProhibits: make(map[uuid.UUID]map[string]*roaring.Bitmap),
		uaProhibits:      make(map[uint32]map[string]*roaring.Bitmap),

		pcIndex: make(map[uuid.UUID]uint32),
		pcByIdx: make([]uuid.UUID, 0),
		uaToPCs: make(map[uint32]*roaring.Bitmap),
		oaToPCs: make(map[uint32]*roaring.Bitmap),
	}

	// 1) Load UA/OA tables to create dense indices
	var uas []postgres.SubjectAttribute
	if err := db.WithContext(ctx).
		Model(&postgres.SubjectAttribute{}).
		Where("tenant_id = ?", tenantID).
		Find(&uas).Error; err != nil {
		return nil, err
	}
	for _, ua := range uas {
		idx := uint32(len(s.uaByIdx))
		s.uaByIdx = append(s.uaByIdx, ua.ID)
		s.uaIndex[ua.ID] = idx
	}

	var oas []postgres.ObjectAttribute
	if err := db.WithContext(ctx).
		Model(&postgres.ObjectAttribute{}).
		Where("tenant_id = ?", tenantID).
		Find(&oas).Error; err != nil {
		return nil, err
	}
	for _, oa := range oas {
		idx := uint32(len(s.oaByIdx))
		s.oaByIdx = append(s.oaByIdx, oa.ID)
		s.oaIndex[oa.ID] = idx
	}

	// Load policy classes
	var pcs []postgres.PolicyClass
	if err := db.WithContext(ctx).
		Model(&postgres.PolicyClass{}).
		Where("tenant_id = ?", tenantID).
		Find(&pcs).Error; err != nil {
		return nil, err
	}
	s.pcIndex = make(map[uuid.UUID]uint32, len(pcs))
	for _, pc := range pcs {
		idx := uint32(len(s.pcByIdx))
		s.pcByIdx = append(s.pcByIdx, pc.ID)
		s.pcIndex[pc.ID] = idx
	}

	// 2) Load assignment edges (typed)
	var edges []postgres.AssignmentEdge
	if err := db.WithContext(ctx).
		Model(&postgres.AssignmentEdge{}).
		Where("tenant_id = ?", tenantID).
		Find(&edges).Error; err != nil {
		return nil, err
	}

	for _, e := range edges {
		switch {
		// subject -> UA
		case e.ChildType == postgres.NodeSubject && e.ParentType == postgres.NodeUA:
			uaIdx, ok := s.uaIndex[e.ParentID]
			if !ok {
				continue
			}
			s.subjectToUAs[e.ChildID] = append(s.subjectToUAs[e.ChildID], uaIdx)
			s.uaDirectSubjects[uaIdx] = append(s.uaDirectSubjects[uaIdx], e.ChildID)

		// UA -> UA
		case e.ChildType == postgres.NodeUA && e.ParentType == postgres.NodeUA:
			c, ok1 := s.uaIndex[e.ChildID]
			p, ok2 := s.uaIndex[e.ParentID]
			if !ok1 || !ok2 {
				continue
			}
			s.uaParents[c] = append(s.uaParents[c], p)
			s.uaChildren[p] = append(s.uaChildren[p], c)

		// object -> OA
		case e.ChildType == postgres.NodeObject && e.ParentType == postgres.NodeOA:
			oaIdx, ok := s.oaIndex[e.ParentID]
			if !ok {
				continue
			}
			s.objectToOAs[e.ChildID] = append(s.objectToOAs[e.ChildID], oaIdx)
			s.oaDirectObjects[oaIdx] = append(s.oaDirectObjects[oaIdx], e.ChildID)

		// OA -> OA
		case e.ChildType == postgres.NodeOA && e.ParentType == postgres.NodeOA:
			c, ok1 := s.oaIndex[e.ChildID]
			p, ok2 := s.oaIndex[e.ParentID]
			if !ok1 || !ok2 {
				continue
			}
			s.oaParents[c] = append(s.oaParents[c], p)
			s.oaChildren[p] = append(s.oaChildren[p], c)

			// UA -> PC
		case e.ChildType == postgres.NodeUA && e.ParentType == postgres.NodePolicyClass:
			uaIdx, ok1 := s.uaIndex[e.ChildID]
			pcIdx, ok2 := s.pcIndex[e.ParentID]
			if !ok1 || !ok2 {
				continue
			}
			if s.uaToPCs[uaIdx] == nil {
				s.uaToPCs[uaIdx] = roaring.New()
			}
			s.uaToPCs[uaIdx].Add(pcIdx)

		// OA -> PC
		case e.ChildType == postgres.NodeOA && e.ParentType == postgres.NodePolicyClass:
			oaIdx, ok1 := s.oaIndex[e.ChildID]
			pcIdx, ok2 := s.pcIndex[e.ParentID]
			if !ok1 || !ok2 {
				continue
			}
			if s.oaToPCs[oaIdx] == nil {
				s.oaToPCs[oaIdx] = roaring.New()
			}
			s.oaToPCs[oaIdx].Add(pcIdx)
		}
	}

	// 3) Load associations + operations
	var assocs []postgres.Association
	if err := db.WithContext(ctx).
		Model(&postgres.Association{}).
		Where("tenant_id = ?", tenantID).
		Find(&assocs).Error; err != nil {
		return nil, err
	}

	// Map associationID -> (uaIdx, oaIdx)
	type assocKey struct{ ua, oa uint32 }
	assocMap := make(map[uuid.UUID]assocKey, len(assocs))
	for _, a := range assocs {
		uaIdx, ok1 := s.uaIndex[a.SubjectAttributeID]
		oaIdx, ok2 := s.oaIndex[a.ObjectAttributeID]
		if !ok1 || !ok2 {
			continue
		}
		assocMap[a.ID] = assocKey{ua: uaIdx, oa: oaIdx}
	}

	var assocOps []postgres.AssociationOperation
	if err := db.WithContext(ctx).
		Model(&postgres.AssociationOperation{}).
		Where("tenant_id = ?", tenantID).
		Find(&assocOps).Error; err != nil {
		return nil, err
	}

	for _, ao := range assocOps {
		k, ok := assocMap[ao.AssociationID]
		if !ok {
			continue
		}
		if s.assoc[k.ua] == nil {
			s.assoc[k.ua] = make(map[string]*roaring.Bitmap)
		}
		if s.assoc[k.ua][ao.Operation] == nil {
			s.assoc[k.ua][ao.Operation] = roaring.New()
		}
		s.assoc[k.ua][ao.Operation].Add(k.oa)
	}

	// 4) Load prohibitions + operations
	var prohs []postgres.Prohibition
	if err := db.WithContext(ctx).
		Model(&postgres.Prohibition{}).
		Where("tenant_id = ?", tenantID).
		Find(&prohs).Error; err != nil {
		return nil, err
	}

	// prohibitionID -> (subjectType, subjectID, oaIdx)
	type prohKey struct {
		subjType postgres.ProhibitionSubjectType
		subjID   uuid.UUID
		oa       uint32
	}
	prohMap := make(map[uuid.UUID]prohKey, len(prohs))
	for _, p := range prohs {
		oaIdx, ok := s.oaIndex[p.ObjectAttributeID]
		if !ok {
			continue
		}
		prohMap[p.ID] = prohKey{subjType: p.SubjectType, subjID: p.SubjectID, oa: oaIdx}
	}

	var prohOps []postgres.ProhibitionOperation
	if err := db.WithContext(ctx).
		Model(&postgres.ProhibitionOperation{}).
		Where("tenant_id = ?", tenantID).
		Find(&prohOps).Error; err != nil {
		return nil, err
	}

	for _, po := range prohOps {
		pk, ok := prohMap[po.ProhibitionID]
		if !ok {
			continue
		}
		switch pk.subjType {
		case postgres.ProhibitSubject:
			if s.subjectProhibits[pk.subjID] == nil {
				s.subjectProhibits[pk.subjID] = make(map[string]*roaring.Bitmap)
			}
			if s.subjectProhibits[pk.subjID][po.Operation] == nil {
				s.subjectProhibits[pk.subjID][po.Operation] = roaring.New()
			}
			s.subjectProhibits[pk.subjID][po.Operation].Add(pk.oa)

		case postgres.ProhibitUA:
			uaIdx, ok := s.uaIndex[pk.subjID]
			if !ok {
				continue
			}
			if s.uaProhibits[uaIdx] == nil {
				s.uaProhibits[uaIdx] = make(map[string]*roaring.Bitmap)
			}
			if s.uaProhibits[uaIdx][po.Operation] == nil {
				s.uaProhibits[uaIdx][po.Operation] = roaring.New()
			}
			s.uaProhibits[uaIdx][po.Operation].Add(pk.oa)
		}
	}

	return s, nil
}
