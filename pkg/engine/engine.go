package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/cache"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/internal/postgres"
)

// Config holds engine configuration
type Config struct {
	TenantID          string            `yaml:"tenant_id" json:"tenant_id"`
	MaxTraversalNodes int               `yaml:"max_traversal_nodes" json:"max_traversal_nodes"` // Maximum nodes to traverse in BFS operations (default: 100000)
	CacheTTL          time.Duration     `yaml:"cache_ttl" json:"cache_ttl"`                     // Default TTL for most caches (default: 2 minutes)
	Limits            cache.CacheLimits `yaml:"limits" json:"limits"`                           // Cache capacity limits (zero = use defaults)
}

type Engine struct {
	log      *logger.Handler
	metric   *metrics.Handler
	db       *postgres.Handler
	tenantID string

	maxTraversalNodes int // Maximum nodes to traverse in BFS operations

	cur atomic.Pointer[Snapshot]

	// Caches - injected via dependency injection
	caches *cache.Caches

	// To lock in the refresh operation
	refreshMu sync.Mutex

	emitter ObligationEmitter
}

// New creates a new engine instance with caches created via dependency injection
// If caches is nil, default caches will be created based on config
func New(log *logger.Handler, metric *metrics.Handler, db *postgres.Handler, cfg *Config, emitter ObligationEmitter, caches *cache.Caches) *Engine {
	maxNodes := cfg.MaxTraversalNodes
	if maxNodes <= 0 {
		maxNodes = 100000 // Default: high limit to not break normal usage
	}

	// Set cache TTL defaults
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 2 * time.Minute // Default TTL
	}

	// Create caches if not provided (dependency injection)
	if caches == nil {
		caches = cache.NewCaches(cacheTTL, cfg.Limits)
	}

	e := &Engine{
		log:               log,
		metric:            metric,
		db:                db,
		tenantID:          cfg.TenantID,
		maxTraversalNodes: maxNodes,
		caches:            caches,
		emitter:           emitter,
	}
	return e
}

// GetDB returns the database handler (for use by HTTP handlers)
func (e *Engine) GetDB() *postgres.Handler {
	return e.db
}

// GetTenantID returns the tenant ID from engine config
func (e *Engine) GetTenantID() string {
	return e.tenantID
}

// Refresh rebuilds the in-memory snapshot and swaps it atomically.
func (e *Engine) Refresh(ctx context.Context) error {
	snap, err := LoadSnapshot(ctx, e.db.H, e.tenantID)
	if err != nil {
		return err
	}

	var rev postgres.PolicyRevision
	_ = e.db.H.WithContext(ctx).Model(&postgres.PolicyRevision{}).First(&rev, "tenant_id = ?", e.tenantID).Error
	snap.Version = rev.Revision

	var maxSeq int64
	e.db.H.WithContext(ctx).
		Model(&postgres.PolicyChange{}).
		Where("tenant_id = ?", e.tenantID).
		Select("COALESCE(MAX(seq), 0)").
		Scan(&maxSeq)
	snap.LastSeq = maxSeq

	e.cur.Store(snap)

	// Invalidate caches on policy change (simple + safe).
	e.caches.UACache.Clear()
	e.caches.OACache.Clear()
	e.caches.AllowCache.Clear()
	e.caches.DenyCache.Clear()
	e.caches.UANodeClosure.Clear()
	e.caches.OANodeClosure.Clear()
	e.caches.Decisions.Clear()
	return nil
}

func (e *Engine) Snapshot() *Snapshot {
	s := e.cur.Load()
	return s
}

// --- Decision (uses closures + bitmaps + cache) ---
// AllowedFor returns the set of OAs allowed for a subject/operation (public for explain endpoint)
func (e *Engine) AllowedFor(s *Snapshot, subject uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	return e.allowedFor(s, subject, op, uaClosure)
}

// AllowedForWithMatches returns the set of OAs allowed and the UA->OA pairs that contributed
// Returns: (allowed bitmap, map of UA ID -> []OA ID that matched)
func (e *Engine) AllowedForWithMatches(s *Snapshot, subject uuid.UUID, op string, uaClosure *roaring.Bitmap) (*roaring.Bitmap, map[uuid.UUID][]uuid.UUID) {
	allowed := roaring.New()
	matches := make(map[uuid.UUID][]uuid.UUID) // UA ID -> []OA ID

	it := uaClosure.Iterator()
	for it.HasNext() {
		uaIdx := it.Next()
		uaID := s.uaByIdx[uaIdx]
		targets := s.assoc[uaIdx][op] // bitmap of OA targets
		if targets != nil {
			oaIDs := []uuid.UUID{}
			tit := targets.Iterator()
			for tit.HasNext() {
				oaIdx := tit.Next()
				oaID := s.oaByIdx[oaIdx]
				oaIDs = append(oaIDs, oaID)
				allowed.Or(e.oaNodeDescendants(s, oaIdx))
			}
			if len(oaIDs) > 0 {
				matches[uaID] = oaIDs
			}
		}
	}
	return allowed, matches
}

func (e *Engine) allowedFor(s *Snapshot, subject uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	if b, ok := e.caches.AllowCache.Get(subject, op); ok {
		return b
	}
	allowed := roaring.New()
	it := uaClosure.Iterator()
	for it.HasNext() {
		ua := it.Next()
		targets := s.assoc[ua][op] // bitmap of OA targets
		if targets != nil {
			it := targets.Iterator()
			for it.HasNext() {
				t := it.Next()
				allowed.Or(e.oaNodeDescendants(s, t))
			}
		}
	}
	// Store immutable bitmap (don't mutate later)
	e.caches.AllowCache.Put(subject, op, allowed)
	return allowed
}

// DeniedFor returns the set of OAs denied for a subject/operation (public for explain endpoint)
func (e *Engine) DeniedFor(s *Snapshot, subject uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	return e.deniedFor(s, subject, op, uaClosure)
}

// DeniedForWithMatches returns the set of OAs denied and the subject->OA pairs that contributed
// Returns: (denied bitmap, map of subject ID -> []OA ID that matched, map of UA ID -> []OA ID that matched)
func (e *Engine) DeniedForWithMatches(s *Snapshot, subject uuid.UUID, op string, uaClosure *roaring.Bitmap) (*roaring.Bitmap, map[uuid.UUID][]uuid.UUID, map[uuid.UUID][]uuid.UUID) {
	denied := roaring.New()
	subjectMatches := make(map[uuid.UUID][]uuid.UUID) // Subject ID -> []OA ID
	uaMatches := make(map[uuid.UUID][]uuid.UUID)      // UA ID -> []OA ID

	// UA-level prohibitions
	it := uaClosure.Iterator()
	for it.HasNext() {
		uaIdx := it.Next()
		uaID := s.uaByIdx[uaIdx]
		opMap := s.uaProhibits[uaIdx]
		if opMap == nil {
			continue
		}
		targets := opMap[op] // bitmap of OA targets
		if targets == nil {
			continue
		}
		oaIDs := []uuid.UUID{}
		tit := targets.Iterator()
		for tit.HasNext() {
			oaIdx := tit.Next()
			oaID := s.oaByIdx[oaIdx]
			oaIDs = append(oaIDs, oaID)
			denied.Or(e.oaNodeDescendants(s, oaIdx))
		}
		if len(oaIDs) > 0 {
			uaMatches[uaID] = oaIDs
		}
	}

	// Subject-level prohibitions
	if opMap := s.subjectProhibits[subject]; opMap != nil {
		if targets := opMap[op]; targets != nil {
			oaIDs := []uuid.UUID{}
			tit := targets.Iterator()
			for tit.HasNext() {
				oaIdx := tit.Next()
				oaID := s.oaByIdx[oaIdx]
				oaIDs = append(oaIDs, oaID)
				denied.Or(e.oaNodeDescendants(s, oaIdx))
			}
			if len(oaIDs) > 0 {
				subjectMatches[subject] = oaIDs
			}
		}
	}

	return denied, subjectMatches, uaMatches
}

func (e *Engine) deniedFor(s *Snapshot, subject uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	if b, ok := e.caches.DenyCache.Get(subject, op); ok {
		return b
	}

	denied := roaring.New()

	// UA-level prohibitions (expand OA targets to descendants)
	it := uaClosure.Iterator()
	for it.HasNext() {
		ua := it.Next()
		opMap := s.uaProhibits[ua]
		if opMap == nil {
			continue
		}
		targets := opMap[op] // bitmap of OA targets
		if targets == nil {
			continue
		}
		tit := targets.Iterator()
		for tit.HasNext() {
			t := tit.Next()
			denied.Or(e.oaNodeDescendants(s, t))
		}
	}

	// subject-level prohibitions (also expand)
	if opMap := s.subjectProhibits[subject]; opMap != nil {
		if targets := opMap[op]; targets != nil {
			tit := targets.Iterator()
			for tit.HasNext() {
				t := tit.Next()
				denied.Or(e.oaNodeDescendants(s, t))
			}
		}
	}

	e.caches.DenyCache.Put(subject, op, denied)
	return denied
}

func (e *Engine) Decide(ctx context.Context, subjectID, objectID uuid.UUID, op string) (bool, error) {
	s := e.Snapshot()
	if s == nil {
		if err := e.Refresh(ctx); err != nil {
			return false, err
		}
		s = e.Snapshot()
	}

	// Decision cache hit?
	if allowed, ok := e.caches.Decisions.Get(subjectID, objectID, op, s.Version); ok {
		return allowed, nil
	}

	uaClosure := e.subjectUAClosure(s, subjectID)
	oaClosure := e.objectOAClosure(s, objectID)

	// PC scope gate
	pcs := commonPCs(s, uaClosure, oaClosure)
	if pcs.IsEmpty() {
		e.caches.Decisions.Put(subjectID, objectID, op, false, s.Version)
		return false, nil
	}

	// Compute subject and object policy classes
	subjectPCs := roaring.New()
	it := uaClosure.Iterator()
	for it.HasNext() {
		ua := it.Next()
		if b := s.uaToPCs[ua]; b != nil {
			subjectPCs.Or(b)
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

	subjectPCs.And(objPCs)
	if subjectPCs.IsEmpty() {
		// No shared policy class => deny (and cache it)
		e.caches.Decisions.Put(subjectID, objectID, op, false, s.Version)
		return false, nil
	}

	allowedBmp := e.allowedFor(s, subjectID, op, uaClosure)
	deniedBmp := e.deniedFor(s, subjectID, op, uaClosure)

	// effective = (allowed - denied) ∩ oaClosure
	effective := allowedBmp.Clone()
	effective.AndNot(deniedBmp)
	effective.And(oaClosure)

	allowed := !effective.IsEmpty()

	e.caches.Decisions.Put(subjectID, objectID, op, allowed, s.Version)
	return allowed, nil
}

func (e *Engine) applyInvalidations(inv *invalidation) {
	// 1) Invalidate node-level closure caches first (UA/OA)
	for uaIdx := range inv.uaNodeClosures {
		e.caches.UANodeClosure.Delete(uaIdx)
	}
	for oaIdx := range inv.oaNodeClosures {
		e.caches.OANodeClosure.Delete(oaIdx)
	}

	// 2) UA closure changes => subject UA-closure + any derived caches become stale
	// (allow/deny/decisions depend on UA closure)
	for u := range inv.subjectsUAClosure {
		e.caches.UACache.Delete(u)
		e.caches.AllowCache.DeleteSubject(u)
		e.caches.DenyCache.DeleteSubject(u)
		e.caches.Decisions.DeleteSubject(u)
	}

	// 3) OA closure changes => object OA-closure + any decisions involving that object become stale
	for o := range inv.objectsOAClosure {
		e.caches.OACache.Delete(o)
		e.caches.Decisions.DeleteObject(o)
	}

	// 4) Dedup ops across allow/deny invalidations (avoid double DeleteSubjectOp calls)
	// opsBySubject[u] = union(inv.subjectAllowOp[u], inv.subjectDenyOp[u])
	opsBySubject := make(map[uuid.UUID]map[string]struct{}, len(inv.subjectAllowOp)+len(inv.subjectDenyOp))

	for u, ops := range inv.subjectAllowOp {
		if opsBySubject[u] == nil {
			opsBySubject[u] = make(map[string]struct{}, len(ops))
		}
		for op := range ops {
			opsBySubject[u][op] = struct{}{}
		}
	}
	for u, ops := range inv.subjectDenyOp {
		if opsBySubject[u] == nil {
			opsBySubject[u] = make(map[string]struct{}, len(ops))
		}
		for op := range ops {
			opsBySubject[u][op] = struct{}{}
		}
	}

	// 5) Invalidate (subject,op) derived caches + decisions
	for u, ops := range opsBySubject {
		for op := range ops {
			// Safe to call even if not present
			e.caches.AllowCache.DeleteSubjectOp(u, op)
			e.caches.DenyCache.DeleteSubjectOp(u, op)
			e.caches.Decisions.DeleteSubjectOp(u, op)
		}
	}
	for u := range inv.subjectsDecisionsOnly {
		e.caches.Decisions.DeleteSubject(u)
	}
	for o := range inv.objectsDecisionsOnly {
		e.caches.Decisions.DeleteObject(o)
	}
}

func (e *Engine) oaNodeDescendants(s *Snapshot, oa uint32) *roaring.Bitmap {
	if b, ok := e.caches.OADescClosure.Get(oa); ok {
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
		for _, ch := range s.oaChildren[cur] {
			if out.CheckedAdd(ch) {
				queue = append(queue, ch)
			}
		}
	}

	e.caches.OADescClosure.Put(oa, out)
	return out
}
