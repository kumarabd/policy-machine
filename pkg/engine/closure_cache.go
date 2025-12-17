package engine

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
)

type cacheEntry struct {
	bmp     *roaring.Bitmap
	expires time.Time
}

type lruElement[K comparable] struct {
	key   K
	entry cacheEntry
}

// closureCache is a bounded TTL cache with LRU eviction
type closureCache[K comparable] struct {
	ttl        time.Duration
	maxEntries int
	mu         sync.Mutex // Single lock for simplicity
	ll         *list.List // LRU list (front = most recent, back = least recent)
	m          map[K]*list.Element

	evictions      atomic.Uint64 // Counter for evictions
	expiredDeletes atomic.Uint64 // Counter for expired entry deletions
}

func newClosureCache[K comparable](ttl time.Duration, maxEntries int) closureCache[K] {
	if maxEntries <= 0 {
		// Default: 50k for UA/OA closures, 200k for node closures
		maxEntries = 50000
	}
	return closureCache[K]{
		ttl:        ttl,
		maxEntries: maxEntries,
		ll:         list.New(),
		m:          make(map[K]*list.Element),
	}
}

// Len returns the current number of entries in the cache
func (c *closureCache[K]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// Get retrieves a bitmap from the cache, updating LRU position if found
func (c *closureCache[K]) Get(k K) (*roaring.Bitmap, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.m[k]
	if !ok {
		return nil, false
	}

	le := elem.Value.(*lruElement[K])
	if now.After(le.entry.expires) {
		// Expired: remove and return miss
		c.ll.Remove(elem)
		delete(c.m, k)
		c.expiredDeletes.Add(1)
		return nil, false
	}

	// Move to front (most recently used)
	c.ll.MoveToFront(elem)
	return le.entry.bmp, true
}

// Put stores a bitmap in the cache, evicting if necessary
func (c *closureCache[K]) Put(k K, bmp *roaring.Bitmap) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expires := now.Add(c.ttl)

	// Check if key already exists
	if elem, ok := c.m[k]; ok {
		// Update existing entry and move to front
		le := elem.Value.(*lruElement[K])
		le.entry.bmp = bmp
		le.entry.expires = expires
		c.ll.MoveToFront(elem)
		return
	}

	// Clean up a few expired entries from the back if possible
	c.cleanupExpiredLocked(5)

	// Check if we need to evict
	for len(c.m) >= c.maxEntries {
		// Evict least recently used (from back)
		back := c.ll.Back()
		if back == nil {
			break
		}
		le := back.Value.(*lruElement[K])
		c.ll.Remove(back)
		delete(c.m, le.key)
		c.evictions.Add(1)
	}

	// Add new entry at front
	le := &lruElement[K]{
		key: k,
		entry: cacheEntry{
			bmp:     bmp,
			expires: expires,
		},
	}
	elem := c.ll.PushFront(le)
	c.m[k] = elem
}

// Delete removes a key from the cache
func (c *closureCache[K]) Delete(k K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.m[k]
	if !ok {
		return
	}
	c.ll.Remove(elem)
	delete(c.m, k)
}

// Clear removes all entries from the cache
func (c *closureCache[K]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ll = list.New()
	c.m = make(map[K]*list.Element)
}

// cleanupExpiredLocked removes up to maxExpired expired entries from the back
// Must be called with lock held
func (c *closureCache[K]) cleanupExpiredLocked(maxExpired int) {
	now := time.Now()
	removed := 0
	for removed < maxExpired {
		back := c.ll.Back()
		if back == nil {
			break
		}
		le := back.Value.(*lruElement[K])
		if now.After(le.entry.expires) {
			c.ll.Remove(back)
			delete(c.m, le.key)
			c.expiredDeletes.Add(1)
			removed++
		} else {
			break // No more expired entries
		}
	}
}

// Evictions returns the number of evictions that have occurred
func (c *closureCache[K]) Evictions() uint64 {
	return c.evictions.Load()
}

// ExpiredDeletes returns the number of expired entries deleted
func (c *closureCache[K]) ExpiredDeletes() uint64 {
	return c.expiredDeletes.Load()
}
