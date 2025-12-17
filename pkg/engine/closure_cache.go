package engine

import (
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring"
)

type cacheEntry struct {
	bmp     *roaring.Bitmap
	expires time.Time
}

type closureCache[K comparable] struct {
	ttl time.Duration
	mu  sync.RWMutex
	m   map[K]cacheEntry
}

func newClosureCache[K comparable](ttl time.Duration) closureCache[K] {
	return closureCache[K]{ttl: ttl, m: make(map[K]cacheEntry)}
}

func (c *closureCache[K]) Get(k K) (*roaring.Bitmap, bool) {
	now := time.Now()
	c.mu.RLock()
	e, ok := c.m[k]
	c.mu.RUnlock()
	if !ok || now.After(e.expires) {
		return nil, false
	}
	return e.bmp, true
}

func (c *closureCache[K]) Put(k K, bmp *roaring.Bitmap) {
	c.mu.Lock()
	c.m[k] = cacheEntry{bmp: bmp, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *closureCache[K]) Delete(k K) {
	c.mu.Lock()
	delete(c.m, k)
	c.mu.Unlock()
}

func (c *closureCache[K]) Clear() {
	c.mu.Lock()
	c.m = make(map[K]cacheEntry)
	c.mu.Unlock()
}
