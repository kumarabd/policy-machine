package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type decisionKey struct {
	User   uuid.UUID
	Object uuid.UUID
	Op     string
}

type decisionEntry struct {
	Allowed  bool
	Expires  time.Time
	Revision int64 // optional; if set, can validate cache against snapshot revision
}

type keySet map[decisionKey]struct{}

// IndexedDecisionCache is a bounded decision cache with LRU eviction and secondary indexes
type IndexedDecisionCache struct {
	ttl        time.Duration
	maxEntries int

	mu sync.RWMutex

	entries  map[decisionKey]decisionEntry
	byUser   map[uuid.UUID]keySet
	byObject map[uuid.UUID]keySet
	byUserOp map[uuid.UUID]map[string]keySet

	// LRU tracking
	ll       *list.List
	lruIndex map[decisionKey]*list.Element

	evictions      atomic.Uint64
	expiredDeletes atomic.Uint64
}

// NewIndexedDecisionCache creates a new indexed decision cache
func NewIndexedDecisionCache(ttl time.Duration, maxEntries int) *IndexedDecisionCache {
	if maxEntries <= 0 {
		maxEntries = 500000 // Default
	}
	return &IndexedDecisionCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		entries:    make(map[decisionKey]decisionEntry),
		byUser:     make(map[uuid.UUID]keySet),
		byObject:   make(map[uuid.UUID]keySet),
		byUserOp:   make(map[uuid.UUID]map[string]keySet),
		ll:         list.New(),
		lruIndex:   make(map[decisionKey]*list.Element),
	}
}

// Len returns the current number of entries in the cache
func (c *IndexedDecisionCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *IndexedDecisionCache) Get(user, object uuid.UUID, op string, curRevision int64) (bool, bool) {
	k := decisionKey{User: user, Object: object, Op: op}
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[k]
	if !ok {
		return false, false
	}

	// Expired? Clean it up lazily.
	if now.After(e.Expires) {
		c.deleteKeyLocked(k)
		return false, false
	}

	// Optional safety check: revision mismatch => treat as miss
	if e.Revision != 0 && e.Revision != curRevision {
		return false, false
	}

	// Update LRU position
	if elem, ok := c.lruIndex[k]; ok {
		c.ll.MoveToFront(elem)
	}

	return e.Allowed, true
}

func (c *IndexedDecisionCache) Put(user, object uuid.UUID, op string, allowed bool, curRevision int64) {
	k := decisionKey{User: user, Object: object, Op: op}
	e := decisionEntry{
		Allowed:  allowed,
		Expires:  time.Now().Add(c.ttl),
		Revision: curRevision,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up a few expired entries
	c.cleanupExpiredLocked(5)

	// Check if we need to evict
	for len(c.entries) >= c.maxEntries {
		back := c.ll.Back()
		if back == nil {
			break
		}
		keyToEvict := back.Value.(decisionKey)
		c.deleteKeyLocked(keyToEvict)
		c.evictions.Add(1)
	}

	// If overwriting an existing key, update LRU position
	if _, exists := c.entries[k]; exists {
		if elem, ok := c.lruIndex[k]; ok {
			c.ll.MoveToFront(elem)
		}
		c.entries[k] = e
		return
	}

	// New entry: add to LRU
	elem := c.ll.PushFront(k)
	c.lruIndex[k] = elem

	// Add to entries
	c.entries[k] = e

	// index: byUser
	if c.byUser[user] == nil {
		c.byUser[user] = make(keySet)
	}
	c.byUser[user][k] = struct{}{}

	// index: byObject
	if c.byObject[object] == nil {
		c.byObject[object] = make(keySet)
	}
	c.byObject[object][k] = struct{}{}

	// index: byUserOp
	if c.byUserOp[user] == nil {
		c.byUserOp[user] = make(map[string]keySet)
	}
	if c.byUserOp[user][op] == nil {
		c.byUserOp[user][op] = make(keySet)
	}
	c.byUserOp[user][op][k] = struct{}{}
}

func (c *IndexedDecisionCache) Delete(user, object uuid.UUID, op string) {
	k := decisionKey{User: user, Object: object, Op: op}
	c.mu.Lock()
	c.deleteKeyLocked(k)
	c.mu.Unlock()
}

func (c *IndexedDecisionCache) DeleteUser(user uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := c.byUser[user]
	if keys == nil {
		return
	}
	for k := range keys {
		c.deleteKeyLocked(k)
	}
	// deleteKeyLocked will also remove c.byUser[user] when empty
}

func (c *IndexedDecisionCache) DeleteObject(object uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := c.byObject[object]
	if keys == nil {
		return
	}
	for k := range keys {
		c.deleteKeyLocked(k)
	}
	// deleteKeyLocked will also remove c.byObject[object] when empty
}

func (c *IndexedDecisionCache) DeleteUserOp(user uuid.UUID, op string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ops := c.byUserOp[user]
	if ops == nil {
		return
	}
	keys := ops[op]
	if keys == nil {
		return
	}
	for k := range keys {
		c.deleteKeyLocked(k)
	}
	// deleteKeyLocked will clean up empty maps
}

func (c *IndexedDecisionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[decisionKey]decisionEntry)
	c.byUser = make(map[uuid.UUID]keySet)
	c.byObject = make(map[uuid.UUID]keySet)
	c.byUserOp = make(map[uuid.UUID]map[string]keySet)
	c.ll = list.New()
	c.lruIndex = make(map[decisionKey]*list.Element)
}

// cleanupExpiredLocked removes up to maxExpired expired entries from the back
// Must be called with lock held
func (c *IndexedDecisionCache) cleanupExpiredLocked(maxExpired int) {
	now := time.Now()
	removed := 0
	for removed < maxExpired {
		back := c.ll.Back()
		if back == nil {
			break
		}
		k := back.Value.(decisionKey)
		e, ok := c.entries[k]
		if !ok || now.After(e.Expires) {
			c.deleteKeyLocked(k)
			c.expiredDeletes.Add(1)
			removed++
		} else {
			break // No more expired entries
		}
	}
}

// --- internal helpers (must be called under write lock) ---

func (c *IndexedDecisionCache) deleteKeyLocked(k decisionKey) {
	// If not present, nothing to do.
	if _, ok := c.entries[k]; !ok {
		return
	}
	delete(c.entries, k)

	// Remove from LRU
	if elem, ok := c.lruIndex[k]; ok {
		c.ll.Remove(elem)
		delete(c.lruIndex, k)
	}

	// byUser cleanup
	if set := c.byUser[k.User]; set != nil {
		delete(set, k)
		if len(set) == 0 {
			delete(c.byUser, k.User)
		}
	}

	// byObject cleanup
	if set := c.byObject[k.Object]; set != nil {
		delete(set, k)
		if len(set) == 0 {
			delete(c.byObject, k.Object)
		}
	}

	// byUserOp cleanup
	if ops := c.byUserOp[k.User]; ops != nil {
		if set := ops[k.Op]; set != nil {
			delete(set, k)
			if len(set) == 0 {
				delete(ops, k.Op)
			}
		}
		if len(ops) == 0 {
			delete(c.byUserOp, k.User)
		}
	}
}

// Evictions returns the number of evictions that have occurred
func (c *IndexedDecisionCache) Evictions() uint64 {
	return c.evictions.Load()
}

// ExpiredDeletes returns the number of expired entries deleted
func (c *IndexedDecisionCache) ExpiredDeletes() uint64 {
	return c.expiredDeletes.Load()
}

