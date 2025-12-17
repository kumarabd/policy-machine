package engine

import (
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/google/uuid"
)

type bmpEntry struct {
	bmp     *roaring.Bitmap
	expires time.Time
}

type userOpBitmapCache struct {
	ttl time.Duration
	mu  sync.RWMutex
	m   map[uuid.UUID]map[string]bmpEntry
}

func newUserOpBitmapCache(ttl time.Duration) *userOpBitmapCache {
	return &userOpBitmapCache{ttl: ttl, m: make(map[uuid.UUID]map[string]bmpEntry)}
}

func (c *userOpBitmapCache) Get(user uuid.UUID, op string) (*roaring.Bitmap, bool) {
	now := time.Now()
	c.mu.RLock()
	ops, ok := c.m[user]
	if !ok {
		c.mu.RUnlock()
		return nil, false
	}
	e, ok := ops[op]
	c.mu.RUnlock()
	if !ok || now.After(e.expires) {
		return nil, false
	}
	return e.bmp, true
}

func (c *userOpBitmapCache) Put(user uuid.UUID, op string, bmp *roaring.Bitmap) {
	c.mu.Lock()
	if c.m[user] == nil {
		c.m[user] = make(map[string]bmpEntry)
	}
	c.m[user][op] = bmpEntry{bmp: bmp, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *userOpBitmapCache) DeleteUser(user uuid.UUID) {
	c.mu.Lock()
	delete(c.m, user)
	c.mu.Unlock()
}

func (c *userOpBitmapCache) DeleteUserOp(user uuid.UUID, op string) {
	c.mu.Lock()
	if c.m[user] != nil {
		delete(c.m[user], op)
		if len(c.m[user]) == 0 {
			delete(c.m, user)
		}
	}
	c.mu.Unlock()
}

func (c *userOpBitmapCache) Clear() {
	c.mu.Lock()
	c.m = make(map[uuid.UUID]map[string]bmpEntry)
	c.mu.Unlock()
}
