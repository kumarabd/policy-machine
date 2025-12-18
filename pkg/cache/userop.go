package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
)

type bmpEntry struct {
	bmp     *roaring.Bitmap
	expires time.Time
}

type userOpKey struct {
	user uuid.UUID
	op   string
}

// UserOpBitmapCache is a bounded nested cache with LRU eviction
type UserOpBitmapCache struct {
	ttl         time.Duration
	maxUsers    int
	maxEntries  int
	mu          sync.Mutex
	m           map[uuid.UUID]map[string]bmpEntry // Main storage
	ll          *list.List                        // LRU list for (user,op) pairs
	lruIndex    map[userOpKey]*list.Element       // Index: (user,op) -> list element

	evictions      atomic.Uint64
	expiredDeletes atomic.Uint64
}

// NewUserOpBitmapCache creates a new user-operation bitmap cache
func NewUserOpBitmapCache(ttl time.Duration, maxUsers, maxEntries int) *UserOpBitmapCache {
	if maxUsers <= 0 {
		maxUsers = 10000 // Default
	}
	if maxEntries <= 0 {
		maxEntries = 100000 // Default
	}
	return &UserOpBitmapCache{
		ttl:        ttl,
		maxUsers:   maxUsers,
		maxEntries: maxEntries,
		m:          make(map[uuid.UUID]map[string]bmpEntry),
		ll:         list.New(),
		lruIndex:   make(map[userOpKey]*list.Element),
	}
}

// LenUsers returns the number of unique users in the cache
func (c *UserOpBitmapCache) LenUsers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// LenEntries returns the total number of (user,op) entries
func (c *UserOpBitmapCache) LenEntries() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, ops := range c.m {
		total += len(ops)
	}
	return total
}

func (c *UserOpBitmapCache) Get(user uuid.UUID, op string) (*roaring.Bitmap, bool) {
	now := time.Now()
	key := userOpKey{user: user, op: op}

	c.mu.Lock()
	defer c.mu.Unlock()

	ops, ok := c.m[user]
	if !ok {
		return nil, false
	}
	e, ok := ops[op]
	if !ok {
		return nil, false
	}

	// Check expiration
	if now.After(e.expires) {
		// Expired: remove
		delete(ops, op)
		if len(ops) == 0 {
			delete(c.m, user)
		}
		if elem, ok := c.lruIndex[key]; ok {
			c.ll.Remove(elem)
			delete(c.lruIndex, key)
		}
		c.expiredDeletes.Add(1)
		return nil, false
	}

	// Update LRU position
	if elem, ok := c.lruIndex[key]; ok {
		c.ll.MoveToFront(elem)
	}

	return e.bmp, true
}

func (c *UserOpBitmapCache) Put(user uuid.UUID, op string, bmp *roaring.Bitmap) {
	now := time.Now()
	key := userOpKey{user: user, op: op}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up a few expired entries
	c.cleanupExpiredLocked(5)

	// Check if entry exists
	if ops, ok := c.m[user]; ok {
		if _, exists := ops[op]; exists {
			// Update existing entry
			ops[op] = bmpEntry{bmp: bmp, expires: now.Add(c.ttl)}
			if elem, ok := c.lruIndex[key]; ok {
				c.ll.MoveToFront(elem)
			}
			return
		}
	} else {
		// New user
		c.m[user] = make(map[string]bmpEntry)
	}

	// Check capacity limits
	// First check entry limit (evicting entries may remove empty users)
	totalEntries := 0
	for _, ops := range c.m {
		totalEntries += len(ops)
	}
	for totalEntries >= c.maxEntries {
		c.evictLRUEntryLocked()
		totalEntries--
	}

	// Then check user limit (if still over, evict a user)
	if len(c.m) >= c.maxUsers {
		c.evictLRUUserLocked()
	}

	// Add new entry
	c.m[user][op] = bmpEntry{bmp: bmp, expires: now.Add(c.ttl)}
	elem := c.ll.PushFront(key)
	c.lruIndex[key] = elem
}

func (c *UserOpBitmapCache) DeleteUser(user uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ops, ok := c.m[user]
	if !ok {
		return
	}

	// Remove all (user,op) entries from LRU
	for op := range ops {
		key := userOpKey{user: user, op: op}
		if elem, ok := c.lruIndex[key]; ok {
			c.ll.Remove(elem)
			delete(c.lruIndex, key)
		}
	}

	delete(c.m, user)
}

func (c *UserOpBitmapCache) DeleteUserOp(user uuid.UUID, op string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ops := c.m[user]
	if ops == nil {
		return
	}

	key := userOpKey{user: user, op: op}
	delete(ops, op)
	if len(ops) == 0 {
		delete(c.m, user)
	}

	// Remove from LRU
	if elem, ok := c.lruIndex[key]; ok {
		c.ll.Remove(elem)
		delete(c.lruIndex, key)
	}
}

func (c *UserOpBitmapCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m = make(map[uuid.UUID]map[string]bmpEntry)
	c.ll = list.New()
	c.lruIndex = make(map[userOpKey]*list.Element)
}

// evictLRUUserLocked evicts the least recently used user and all their entries
// Must be called with lock held
func (c *UserOpBitmapCache) evictLRUUserLocked() {
	// Find a user by looking at the back of the LRU list
	// Keep going until we find a user and remove all their entries
	seenUsers := make(map[uuid.UUID]struct{})
	
	for c.ll.Len() > 0 && len(c.m) >= c.maxUsers {
		back := c.ll.Back()
		if back == nil {
			break
		}
		key := back.Value.(userOpKey)
		
		// Skip if we've already processed this user
		if _, seen := seenUsers[key.user]; seen {
			// Move to front temporarily to avoid infinite loop
			c.ll.MoveToFront(back)
			break
		}
		seenUsers[key.user] = struct{}{}
		
		// Remove all entries for this user
		ops := c.m[key.user]
		if ops == nil {
			continue
		}
		
		entryCount := len(ops)
		for op := range ops {
			k := userOpKey{user: key.user, op: op}
			if elem, ok := c.lruIndex[k]; ok {
				c.ll.Remove(elem)
				delete(c.lruIndex, k)
			}
		}
		delete(c.m, key.user)
		c.evictions.Add(uint64(entryCount))
		
		if len(c.m) < c.maxUsers {
			break
		}
	}
}

// evictLRUEntryLocked evicts the least recently used (user,op) entry
// Must be called with lock held
func (c *UserOpBitmapCache) evictLRUEntryLocked() {
	back := c.ll.Back()
	if back == nil {
		return
	}

	key := back.Value.(userOpKey)
	c.ll.Remove(back)
	delete(c.lruIndex, key)

	ops := c.m[key.user]
	if ops != nil {
		delete(ops, key.op)
		if len(ops) == 0 {
			delete(c.m, key.user)
		}
	}

	c.evictions.Add(1)
}

// cleanupExpiredLocked removes up to maxExpired expired entries from the back
// Must be called with lock held
func (c *UserOpBitmapCache) cleanupExpiredLocked(maxExpired int) {
	now := time.Now()
	removed := 0
	for removed < maxExpired {
		back := c.ll.Back()
		if back == nil {
			break
		}
		key := back.Value.(userOpKey)
		ops := c.m[key.user]
		if ops == nil {
			c.ll.Remove(back)
			delete(c.lruIndex, key)
			removed++
			continue
		}
		entry, ok := ops[key.op]
		if !ok || now.After(entry.expires) {
			c.ll.Remove(back)
			delete(c.lruIndex, key)
			delete(ops, key.op)
			if len(ops) == 0 {
				delete(c.m, key.user)
			}
			c.expiredDeletes.Add(1)
			removed++
		} else {
			break // No more expired entries
		}
	}
}

// Evictions returns the number of evictions that have occurred
func (c *UserOpBitmapCache) Evictions() uint64 {
	return c.evictions.Load()
}

// ExpiredDeletes returns the number of expired entries deleted
func (c *UserOpBitmapCache) ExpiredDeletes() uint64 {
	return c.expiredDeletes.Load()
}

