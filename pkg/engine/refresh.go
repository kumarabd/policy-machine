package engine

import (
	"context"

	"github.com/RoaringBitmap/roaring"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"gorm.io/gorm"
)

func (e *Engine) RefreshIncremental(ctx context.Context) error {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	s := e.Snapshot()
	if s == nil {
		// bootstrap
		return e.Refresh(ctx)
	}

	// 1) Read current revision
	var rev postgres.PolicyRevision
	if err := e.db.H.WithContext(ctx).First(&rev, "tenant_id = ?", e.tenantID).Error; err != nil {
		// If row doesn't exist, nothing to do
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	if rev.Revision <= s.Version {
		return nil // up to date
	}

	// 2) Fetch deltas since last seq (ordered)
	var changes []postgres.PolicyChange
	if err := e.db.H.WithContext(ctx).
		Where("tenant_id = ? AND seq > ?", e.tenantID, s.LastSeq).
		Order("seq ASC").
		Find(&changes).Error; err != nil {
		return err
	}

	// If revision advanced but we got no changes -> treat as gap/retention/replica lag => rebuild
	if len(changes) == 0 {
		return e.Refresh(ctx)
	}
	// Gap detection
	if !validateSeqContinuity(s.LastSeq, changes) {
		return e.Refresh(ctx)
	}

	// 3) Copy-on-write snapshot and apply
	inv := newInvalidation()
	next := cloneSnapshotShallow(s) // see below
	lastRevision := s.Version
	for _, ch := range changes {
		// Revision monotonic check (optional but recommended)
		if ch.Revision < lastRevision {
			return e.Refresh(ctx)
		}
		lastRevision = ch.Revision

		computeInvalidations(inv, s, ch) // s = old/current snapshot
		if err := applyChange(next, ch); err != nil {
			// if anything feels risky -> full rebuild fallback
			return e.Refresh(ctx)
		}
		next.LastSeq = ch.Seq
		next.Version = ch.Revision
	}

	// 4) Atomically swap + clear closure caches
	e.cur.Store(next)
	e.computeSubjectOpInvalidationsFromOADescChanges(s, inv)
	e.applyInvalidations(inv)
	e.caches.UACache.Clear()
	e.caches.OACache.Clear()

	go e.Warmup(next, inv) // or call synchronously if you prefer deterministic behavior

	return nil
}

func (e *Engine) Warmup(s *Snapshot, inv *invalidation) {
	// Warm subject closures
	for u := range inv.subjectsUAClosure {
		ua := e.subjectUAClosure(s, u)
		_ = ua
		// If you track which ops were affected in inv.subjectAllowOp/subjectDenyOp:
		for op := range inv.subjectAllowOp[u] {
			e.allowedFor(s, u, op, ua)
		}
		for op := range inv.subjectDenyOp[u] {
			e.deniedFor(s, u, op, ua)
		}
	}

	// Warm object closures
	for o := range inv.objectsOAClosure {
		oa := e.objectOAClosure(s, o)
		_ = oa
	}
}

func cloneSnapshotShallow(s *Snapshot) *Snapshot {
	// Shallow copy struct; maps will still alias.
	// For correctness, in applyChange we clone ONLY the specific
	// bitmaps / map entries we mutate (copy-on-write per key).
	cp := *s
	return &cp
}

func validateSeqContinuity(lastSeq int64, changes []postgres.PolicyChange) bool {
	if len(changes) == 0 {
		return true
	}
	if lastSeq != 0 && changes[0].Seq != lastSeq+1 {
		return false
	}
	for i := 1; i < len(changes); i++ {
		if changes[i].Seq != changes[i-1].Seq+1 {
			return false
		}
	}
	return true
}

func (e *Engine) computeSubjectOpInvalidationsFromOADescChanges(s *Snapshot, inv *invalidation) {
	if len(inv.oaDescClosures) == 0 {
		return
	}

	affected := roaring.New()
	for oa := range inv.oaDescClosures {
		affected.Add(oa)
	}

	// Associations: if UA has targets intersecting affected OA nodes, invalidate allow for subjects in UA subtree.
	for uaIdx, opMap := range s.assoc {
		for op, targets := range opMap {
			if targets == nil || !targets.Intersects(affected) {
				continue
			}
			for _, u := range s.SubjectsInUASubtree(uaIdx) {
				if inv.subjectAllowOp[u] == nil {
					inv.subjectAllowOp[u] = map[string]struct{}{}
				}
				inv.subjectAllowOp[u][op] = struct{}{}
			}
		}
	}

	// UA prohibitions: invalidate deny for subjects in UA subtree if targets intersect affected.
	for uaIdx, opMap := range s.uaProhibits {
		for op, targets := range opMap {
			if targets == nil || !targets.Intersects(affected) {
				continue
			}
			for _, u := range s.SubjectsInUASubtree(uaIdx) {
				if inv.subjectDenyOp[u] == nil {
					inv.subjectDenyOp[u] = map[string]struct{}{}
				}
				inv.subjectDenyOp[u][op] = struct{}{}
			}
		}
	}

	// Subject prohibitions: invalidate deny(subject,op) if targets intersect affected.
	for subjectID, opMap := range s.subjectProhibits {
		for op, targets := range opMap {
			if targets == nil || !targets.Intersects(affected) {
				continue
			}
			if inv.subjectDenyOp[subjectID] == nil {
				inv.subjectDenyOp[subjectID] = map[string]struct{}{}
			}
			inv.subjectDenyOp[subjectID][op] = struct{}{}
		}
	}
}
