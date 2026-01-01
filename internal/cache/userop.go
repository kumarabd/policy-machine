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

type subjectOpKey struct {
	subject uuid.UUID
	op      string
}

// SubjectOpBitmapCache is a bounded nested cache with LRU eviction
type SubjectOpBitmapCache struct {
	ttl         time.Duration
	maxSubjects int
	maxEntries  int
	mu          sync.Mutex
	m           map[uuid.UUID]map[string]bmpEntry // Main storage
	ll          *list.List                        // LRU list for (subject,op) pairs
	lruIndex    map[subjectOpKey]*list.Element    // Index: (subject,op) -> list element

	evictions      atomic.Uint64
	expiredDeletes atomic.Uint64
}

// NewSubjectOpBitmapCache creates a new subject-operation bitmap cache
func NewSubjectOpBitmapCache(ttl time.Duration, maxSubjects, maxEntries int) *SubjectOpBitmapCache {
	if maxSubjects <= 0 {
		maxSubjects = 10000 // Default
	}
	if maxEntries <= 0 {
		maxEntries = 100000 // Default
	}
	return &SubjectOpBitmapCache{
		ttl:         ttl,
		maxSubjects: maxSubjects,
		maxEntries:  maxEntries,
		m:           make(map[uuid.UUID]map[string]bmpEntry),
		ll:          list.New(),
		lruIndex:    make(map[subjectOpKey]*list.Element),
	}
}

// LenSubjects returns the number of unique subjects in the cache
func (c *SubjectOpBitmapCache) LenSubjects() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// LenEntries returns the total number of (subject,op) entries
func (c *SubjectOpBitmapCache) LenEntries() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, ops := range c.m {
		total += len(ops)
	}
	return total
}

func (c *SubjectOpBitmapCache) Get(subject uuid.UUID, op string) (*roaring.Bitmap, bool) {
	now := time.Now()
	key := subjectOpKey{subject: subject, op: op}

	c.mu.Lock()
	defer c.mu.Unlock()

	ops, ok := c.m[subject]
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
			delete(c.m, subject)
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

func (c *SubjectOpBitmapCache) Put(subject uuid.UUID, op string, bmp *roaring.Bitmap) {
	now := time.Now()
	key := subjectOpKey{subject: subject, op: op}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up a few expired entries
	c.cleanupExpiredLocked(5)

	// Check if entry exists
	if ops, ok := c.m[subject]; ok {
		if _, exists := ops[op]; exists {
			// Update existing entry
			ops[op] = bmpEntry{bmp: bmp, expires: now.Add(c.ttl)}
			if elem, ok := c.lruIndex[key]; ok {
				c.ll.MoveToFront(elem)
			}
			return
		}
	} else {
		// New subject
		c.m[subject] = make(map[string]bmpEntry)
	}

	// Check capacity limits
	// First check entry limit (evicting entries may remove empty subjects)
	totalEntries := 0
	for _, ops := range c.m {
		totalEntries += len(ops)
	}
	for totalEntries >= c.maxEntries {
		c.evictLRUEntryLocked()
		totalEntries--
	}

	// Then check subject limit (if still over, evict a subject)
	if len(c.m) >= c.maxSubjects {
		c.evictLRUSubjectLocked()
	}

	// Add new entry
	c.m[subject][op] = bmpEntry{bmp: bmp, expires: now.Add(c.ttl)}
	elem := c.ll.PushFront(key)
	c.lruIndex[key] = elem
}

func (c *SubjectOpBitmapCache) DeleteSubject(subject uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ops, ok := c.m[subject]
	if !ok {
		return
	}

	// Remove all (subject,op) entries from LRU
	for op := range ops {
		key := subjectOpKey{subject: subject, op: op}
		if elem, ok := c.lruIndex[key]; ok {
			c.ll.Remove(elem)
			delete(c.lruIndex, key)
		}
	}

	delete(c.m, subject)
}

func (c *SubjectOpBitmapCache) DeleteSubjectOp(subject uuid.UUID, op string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ops := c.m[subject]
	if ops == nil {
		return
	}

	key := subjectOpKey{subject: subject, op: op}
	delete(ops, op)
	if len(ops) == 0 {
		delete(c.m, subject)
	}

	// Remove from LRU
	if elem, ok := c.lruIndex[key]; ok {
		c.ll.Remove(elem)
		delete(c.lruIndex, key)
	}
}

func (c *SubjectOpBitmapCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m = make(map[uuid.UUID]map[string]bmpEntry)
	c.ll = list.New()
	c.lruIndex = make(map[subjectOpKey]*list.Element)
}

// evictLRUSubjectLocked evicts the least recently used subject and all their entries
// Must be called with lock held
func (c *SubjectOpBitmapCache) evictLRUSubjectLocked() {
	// Find a subject by looking at the back of the LRU list
	// Keep going until we find a subject and remove all their entries
	seenSubjects := make(map[uuid.UUID]struct{})

	for c.ll.Len() > 0 && len(c.m) >= c.maxSubjects {
		back := c.ll.Back()
		if back == nil {
			break
		}
		key := back.Value.(subjectOpKey)

		// Skip if we've already processed this subject
		if _, seen := seenSubjects[key.subject]; seen {
			// Move to front temporarily to avoid infinite loop
			c.ll.MoveToFront(back)
			break
		}
		seenSubjects[key.subject] = struct{}{}

		// Remove all entries for this subject
		ops := c.m[key.subject]
		if ops == nil {
			continue
		}

		entryCount := len(ops)
		for op := range ops {
			k := subjectOpKey{subject: key.subject, op: op}
			if elem, ok := c.lruIndex[k]; ok {
				c.ll.Remove(elem)
				delete(c.lruIndex, k)
			}
		}
		delete(c.m, key.subject)
		c.evictions.Add(uint64(entryCount))

		if len(c.m) < c.maxSubjects {
			break
		}
	}
}

// evictLRUEntryLocked evicts the least recently used (subject,op) entry
// Must be called with lock held
func (c *SubjectOpBitmapCache) evictLRUEntryLocked() {
	back := c.ll.Back()
	if back == nil {
		return
	}

	key := back.Value.(subjectOpKey)
	c.ll.Remove(back)
	delete(c.lruIndex, key)

	ops := c.m[key.subject]
	if ops != nil {
		delete(ops, key.op)
		if len(ops) == 0 {
			delete(c.m, key.subject)
		}
	}

	c.evictions.Add(1)
}

// cleanupExpiredLocked removes up to maxExpired expired entries from the back
// Must be called with lock held
func (c *SubjectOpBitmapCache) cleanupExpiredLocked(maxExpired int) {
	now := time.Now()
	removed := 0
	for removed < maxExpired {
		back := c.ll.Back()
		if back == nil {
			break
		}
		key := back.Value.(subjectOpKey)
		ops := c.m[key.subject]
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
				delete(c.m, key.subject)
			}
			c.expiredDeletes.Add(1)
			removed++
		} else {
			break // No more expired entries
		}
	}
}

// Evictions returns the number of evictions that have occurred
func (c *SubjectOpBitmapCache) Evictions() uint64 {
	return c.evictions.Load()
}

// ExpiredDeletes returns the number of expired entries deleted
func (c *SubjectOpBitmapCache) ExpiredDeletes() uint64 {
	return c.expiredDeletes.Load()
}
