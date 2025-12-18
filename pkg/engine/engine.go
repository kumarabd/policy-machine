package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/postgres"
)

// CacheLimits defines maximum entries for each cache type.
// Zero values use defaults:
//   - UA/OA closure caches: 50,000 entries
//   - Node closure caches: 200,000 entries
//   - OA descendants: 200,000 entries
//   - Allow/Deny caches: 10,000 users, 100,000 (user,op) entries
//   - Decision cache: 500,000 entries
type CacheLimits struct {
	UAClosureMaxEntries     int // Max entries in user->UA closure cache (default: 50k)
	OAClosureMaxEntries     int // Max entries in object->OA closure cache (default: 50k)
	UANodeClosureMaxEntries int // Max entries in UA node closure cache (default: 200k)
	OANodeClosureMaxEntries int // Max entries in OA node closure cache (default: 200k)
	OADescClosureMaxEntries int // Max entries in OA descendants closure cache (default: 200k)

	AllowCacheMaxUsers   int // Max number of user keys in allow cache (default: 10k)
	AllowCacheMaxEntries int // Max total (user,op) pairs in allow cache (default: 100k)
	DenyCacheMaxUsers    int // Max number of user keys in deny cache (default: 10k)
	DenyCacheMaxEntries  int // Max total (user,op) pairs in deny cache (default: 100k)

	DecisionCacheMaxEntries int // Max total decision keys (default: 500k)
}

// Config holds engine configuration
type Config struct {
	TenantID          uuid.UUID     `yaml:"tenant_id" json:"tenant_id"`
	MaxTraversalNodes int           `yaml:"max_traversal_nodes" json:"max_traversal_nodes"` // Maximum nodes to traverse in BFS operations (default: 100000)
	CacheTTL          time.Duration `yaml:"cache_ttl" json:"cache_ttl"`                     // Default TTL for most caches (default: 2 minutes)
	Limits            CacheLimits   `yaml:"limits" json:"limits"`                           // Cache capacity limits (zero = use defaults)
	MockMode          bool          `yaml:"mock_mode" json:"mock_mode"`                     // Enable mock mode for API responses
}

type Engine struct {
	log      *logger.Handler
	metric   *metrics.Handler
	db       *postgres.Handler
	tenantID uuid.UUID

	maxTraversalNodes int // Maximum nodes to traverse in BFS operations

	cur atomic.Pointer[Snapshot]

	// Closure caches (TTL-based). Keep separate for UA/OA.
	uaCache closureCache[uuid.UUID]
	oaCache closureCache[uuid.UUID]

	// UserOP caches (TTL-based).
	allowCache *userOpBitmapCache
	denyCache  *userOpBitmapCache

	uaNodeClosure closureCache[uint32] // uaIdx -> bitmap of reachable UAs (including itself)
	oaNodeClosure closureCache[uint32] // oaIdx -> bitmap of reachable OAs (including itself)
	oaDescClosure closureCache[uint32] // oaIdx -> bitmap(descendants + self)

	decisions *indexedDecisionCache

	// To lock in the refresh operation
	refreshMu sync.Mutex

	emitter ObligationEmitter
}

func New(log *logger.Handler, metric *metrics.Handler, db *postgres.Handler, cfg *Config, emitter ObligationEmitter) *Engine {
	maxNodes := cfg.MaxTraversalNodes
	if maxNodes <= 0 {
		maxNodes = 100000 // Default: high limit to not break normal usage
	}

	// Set cache TTL defaults
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 2 * time.Minute // Default TTL
	}

	// Apply cache limits with defaults
	limits := cfg.Limits
	if limits.UAClosureMaxEntries <= 0 {
		limits.UAClosureMaxEntries = 50000
	}
	if limits.OAClosureMaxEntries <= 0 {
		limits.OAClosureMaxEntries = 50000
	}
	if limits.UANodeClosureMaxEntries <= 0 {
		limits.UANodeClosureMaxEntries = 200000
	}
	if limits.OANodeClosureMaxEntries <= 0 {
		limits.OANodeClosureMaxEntries = 200000
	}
	if limits.OADescClosureMaxEntries <= 0 {
		limits.OADescClosureMaxEntries = 200000
	}
	if limits.AllowCacheMaxUsers <= 0 {
		limits.AllowCacheMaxUsers = 10000
	}
	if limits.AllowCacheMaxEntries <= 0 {
		limits.AllowCacheMaxEntries = 100000
	}
	if limits.DenyCacheMaxUsers <= 0 {
		limits.DenyCacheMaxUsers = 10000
	}
	if limits.DenyCacheMaxEntries <= 0 {
		limits.DenyCacheMaxEntries = 100000
	}
	if limits.DecisionCacheMaxEntries <= 0 {
		limits.DecisionCacheMaxEntries = 500000
	}

	e := &Engine{
		log:               log,
		metric:            metric,
		db:                db,
		tenantID:          cfg.TenantID,
		maxTraversalNodes: maxNodes,
		uaCache:           newClosureCache[uuid.UUID](cacheTTL, limits.UAClosureMaxEntries),
		oaCache:           newClosureCache[uuid.UUID](cacheTTL, limits.OAClosureMaxEntries),
		allowCache:        newUserOpBitmapCache(cacheTTL, limits.AllowCacheMaxUsers, limits.AllowCacheMaxEntries),
		denyCache:         newUserOpBitmapCache(cacheTTL, limits.DenyCacheMaxUsers, limits.DenyCacheMaxEntries),
		uaNodeClosure:     newClosureCache[uint32](10*time.Minute, limits.UANodeClosureMaxEntries),
		oaNodeClosure:     newClosureCache[uint32](10*time.Minute, limits.OANodeClosureMaxEntries),
		oaDescClosure:     newClosureCache[uint32](10*time.Minute, limits.OADescClosureMaxEntries),
		decisions:         newIndexedDecisionCache(60*time.Second, limits.DecisionCacheMaxEntries),
		emitter:           emitter,
	}
	return e
}

// GetDB returns the database handler (for use by HTTP handlers)
func (e *Engine) GetDB() *postgres.Handler {
	return e.db
}

// GetTenantID returns the tenant ID from engine config
func (e *Engine) GetTenantID() uuid.UUID {
	return e.tenantID
}

// Refresh rebuilds the in-memory snapshot and swaps it atomically.
func (e *Engine) Refresh(ctx context.Context) error {
	snap, err := LoadSnapshot(ctx, e.db.H, e.tenantID)
	if err != nil {
		return err
	}

	var rev postgres.PolicyRevision
	_ = e.db.H.WithContext(ctx).First(&rev, "tenant_id = ?", e.tenantID).Error
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
	e.uaCache.Clear()
	e.oaCache.Clear()
	e.allowCache.Clear()
	e.denyCache.Clear()
	e.uaNodeClosure.Clear()
	e.oaNodeClosure.Clear()
	e.decisions.Clear()
	return nil
}

func (e *Engine) Snapshot() *Snapshot {
	s := e.cur.Load()
	return s
}

// --- Decision (uses closures + bitmaps + cache) ---
// AllowedFor returns the set of OAs allowed for a user/operation (public for explain endpoint)
func (e *Engine) AllowedFor(s *Snapshot, user uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	return e.allowedFor(s, user, op, uaClosure)
}

func (e *Engine) allowedFor(s *Snapshot, user uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	if b, ok := e.allowCache.Get(user, op); ok {
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
	// Store immutable bitmap (don’t mutate later)
	e.allowCache.Put(user, op, allowed)
	return allowed
}

// DeniedFor returns the set of OAs denied for a user/operation (public for explain endpoint)
func (e *Engine) DeniedFor(s *Snapshot, user uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	return e.deniedFor(s, user, op, uaClosure)
}

func (e *Engine) deniedFor(s *Snapshot, user uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
	if b, ok := e.denyCache.Get(user, op); ok {
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

	// user-level prohibitions (also expand)
	if opMap := s.userProhibits[user]; opMap != nil {
		if targets := opMap[op]; targets != nil {
			tit := targets.Iterator()
			for tit.HasNext() {
				t := tit.Next()
				denied.Or(e.oaNodeDescendants(s, t))
			}
		}
	}

	e.denyCache.Put(user, op, denied)
	return denied
}

func (e *Engine) Decide(ctx context.Context, userID, objectID uuid.UUID, op string) (bool, error) {
	s := e.Snapshot()
	if s == nil {
		if err := e.Refresh(ctx); err != nil {
			return false, err
		}
		s = e.Snapshot()
	}

	// Decision cache hit?
	if allowed, ok := e.decisions.Get(userID, objectID, op, s.Version); ok {
		return allowed, nil
	}

	uaClosure := e.userUAClosure(s, userID)
	oaClosure := e.objectOAClosure(s, objectID)

	// PC scope gate
	pcs := commonPCs(s, uaClosure, oaClosure)
	if pcs.IsEmpty() {
		e.decisions.Put(userID, objectID, op, false, s.Version)
		return false, nil
	}

	// Compute user and object policy classes
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
	if userPCs.IsEmpty() {
		// No shared policy class => deny (and cache it)
		e.decisions.Put(userID, objectID, op, false, s.Version)
		return false, nil
	}

	allowedBmp := e.allowedFor(s, userID, op, uaClosure)
	deniedBmp := e.deniedFor(s, userID, op, uaClosure)

	// effective = (allowed - denied) ∩ oaClosure
	effective := allowedBmp.Clone()
	effective.AndNot(deniedBmp)
	effective.And(oaClosure)

	allowed := !effective.IsEmpty()

	e.decisions.Put(userID, objectID, op, allowed, s.Version)
	return allowed, nil
}

func (e *Engine) applyInvalidations(inv *invalidation) {
	// 1) Invalidate node-level closure caches first (UA/OA)
	for uaIdx := range inv.uaNodeClosures {
		e.uaNodeClosure.Delete(uaIdx)
	}
	for oaIdx := range inv.oaNodeClosures {
		e.oaNodeClosure.Delete(oaIdx)
	}

	// 2) UA closure changes => user UA-closure + any derived caches become stale
	// (allow/deny/decisions depend on UA closure)
	for u := range inv.usersUAClosure {
		e.uaCache.Delete(u)
		e.allowCache.DeleteUser(u)
		e.denyCache.DeleteUser(u)
		e.decisions.DeleteUser(u)
	}

	// 3) OA closure changes => object OA-closure + any decisions involving that object become stale
	for o := range inv.objectsOAClosure {
		e.oaCache.Delete(o)
		e.decisions.DeleteObject(o)
	}

	// 4) Dedup ops across allow/deny invalidations (avoid double DeleteUserOp calls)
	// opsByUser[u] = union(inv.userAllowOp[u], inv.userDenyOp[u])
	opsByUser := make(map[uuid.UUID]map[string]struct{}, len(inv.userAllowOp)+len(inv.userDenyOp))

	for u, ops := range inv.userAllowOp {
		if opsByUser[u] == nil {
			opsByUser[u] = make(map[string]struct{}, len(ops))
		}
		for op := range ops {
			opsByUser[u][op] = struct{}{}
		}
	}
	for u, ops := range inv.userDenyOp {
		if opsByUser[u] == nil {
			opsByUser[u] = make(map[string]struct{}, len(ops))
		}
		for op := range ops {
			opsByUser[u][op] = struct{}{}
		}
	}

	// 5) Invalidate (user,op) derived caches + decisions
	for u, ops := range opsByUser {
		for op := range ops {
			// Safe to call even if not present
			e.allowCache.DeleteUserOp(u, op)
			e.denyCache.DeleteUserOp(u, op)
			e.decisions.DeleteUserOp(u, op)
		}
	}
	for u := range inv.usersDecisionsOnly {
		e.decisions.DeleteUser(u)
	}
	for o := range inv.objectsDecisionsOnly {
		e.decisions.DeleteObject(o)
	}
}

func (e *Engine) oaNodeDescendants(s *Snapshot, oa uint32) *roaring.Bitmap {
	if b, ok := e.oaDescClosure.Get(oa); ok {
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

	e.oaDescClosure.Put(oa, out)
	return out
}
