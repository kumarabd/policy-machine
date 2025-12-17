package engine

import (
	"sync"
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

type indexedDecisionCache struct {
	ttl time.Duration

	mu sync.RWMutex

	entries  map[decisionKey]decisionEntry
	byUser   map[uuid.UUID]keySet
	byObject map[uuid.UUID]keySet
	byUserOp map[uuid.UUID]map[string]keySet
}

func newIndexedDecisionCache(ttl time.Duration) *indexedDecisionCache {
	return &indexedDecisionCache{
		ttl:      ttl,
		entries:  make(map[decisionKey]decisionEntry),
		byUser:   make(map[uuid.UUID]keySet),
		byObject: make(map[uuid.UUID]keySet),
		byUserOp: make(map[uuid.UUID]map[string]keySet),
	}
}

func (c *indexedDecisionCache) Get(user, object uuid.UUID, op string, curRevision int64) (bool, bool) {
	k := decisionKey{User: user, Object: object, Op: op}
	now := time.Now()

	c.mu.RLock()
	e, ok := c.entries[k]
	c.mu.RUnlock()
	if !ok {
		return false, false
	}

	// Expired? Clean it up lazily.
	if now.After(e.Expires) {
		c.mu.Lock()
		// Re-check under write lock
		if e2, ok2 := c.entries[k]; ok2 && now.After(e2.Expires) {
			c.deleteKeyLocked(k)
		}
		c.mu.Unlock()
		return false, false
	}

	// Optional safety check: revision mismatch => treat as miss
	if e.Revision != 0 && e.Revision != curRevision {
		return false, false
	}

	return e.Allowed, true
}

func (c *indexedDecisionCache) Put(user, object uuid.UUID, op string, allowed bool, curRevision int64) {
	k := decisionKey{User: user, Object: object, Op: op}
	e := decisionEntry{
		Allowed:  allowed,
		Expires:  time.Now().Add(c.ttl),
		Revision: curRevision,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If overwriting an existing key, we can keep indexes as-is (sets prevent dupes).
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

func (c *indexedDecisionCache) Delete(user, object uuid.UUID, op string) {
	k := decisionKey{User: user, Object: object, Op: op}
	c.mu.Lock()
	c.deleteKeyLocked(k)
	c.mu.Unlock()
}

func (c *indexedDecisionCache) DeleteUser(user uuid.UUID) {
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

func (c *indexedDecisionCache) DeleteObject(object uuid.UUID) {
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

func (c *indexedDecisionCache) DeleteUserOp(user uuid.UUID, op string) {
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

func (c *indexedDecisionCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[decisionKey]decisionEntry)
	c.byUser = make(map[uuid.UUID]keySet)
	c.byObject = make(map[uuid.UUID]keySet)
	c.byUserOp = make(map[uuid.UUID]map[string]keySet)
	c.mu.Unlock()
}

// --- internal helpers (must be called under write lock) ---

func (c *indexedDecisionCache) deleteKeyLocked(k decisionKey) {
	// If not present, nothing to do.
	if _, ok := c.entries[k]; !ok {
		return
	}
	delete(c.entries, k)

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
