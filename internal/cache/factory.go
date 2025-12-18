package cache

import (
	"time"

	"github.com/google/uuid"
)

// CacheLimits defines maximum entries for each cache type.
type CacheLimits struct {
	UAClosureMaxEntries     int `yaml:"ua_closure_max_entries" json:"ua_closure_max_entries"`           // Max entries in user->UA closure cache (default: 50k)
	OAClosureMaxEntries     int `yaml:"oa_closure_max_entries" json:"oa_closure_max_entries"`           // Max entries in object->OA closure cache (default: 50k)
	UANodeClosureMaxEntries int `yaml:"ua_node_closure_max_entries" json:"ua_node_closure_max_entries"` // Max entries in UA node closure cache (default: 200k)
	OANodeClosureMaxEntries int `yaml:"oa_node_closure_max_entries" json:"oa_node_closure_max_entries"` // Max entries in OA node closure cache (default: 200k)
	OADescClosureMaxEntries int `yaml:"oa_desc_closure_max_entries" json:"oa_desc_closure_max_entries"` // Max entries in OA descendants closure cache (default: 200k)

	AllowCacheMaxUsers   int `yaml:"allow_cache_max_users" json:"allow_cache_max_users"`     // Max number of user keys in allow cache (default: 10k)
	AllowCacheMaxEntries int `yaml:"allow_cache_max_entries" json:"allow_cache_max_entries"` // Max total (user,op) pairs in allow cache (default: 100k)
	DenyCacheMaxUsers    int `yaml:"deny_cache_max_users" json:"deny_cache_max_users"`       // Max number of user keys in deny cache (default: 10k)
	DenyCacheMaxEntries  int `yaml:"deny_cache_max_entries" json:"deny_cache_max_entries"`   // Max total (user,op) pairs in deny cache (default: 100k)

	DecisionCacheMaxEntries int `yaml:"decision_cache_max_entries" json:"decision_cache_max_entries"` // Max total decision keys (default: 500k)
}

// Caches holds all cache instances for the engine
type Caches struct {
	UACache       *ClosureCache[uuid.UUID]
	OACache       *ClosureCache[uuid.UUID]
	AllowCache    *UserOpBitmapCache
	DenyCache     *UserOpBitmapCache
	UANodeClosure *ClosureCache[uint32]
	OANodeClosure *ClosureCache[uint32]
	OADescClosure *ClosureCache[uint32]
	Decisions     *IndexedDecisionCache
}

// NewCaches creates all cache instances with the specified TTL and limits
func NewCaches(cacheTTL time.Duration, limits CacheLimits) *Caches {
	// Apply defaults
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

	return &Caches{
		UACache:       NewClosureCache[uuid.UUID](cacheTTL, limits.UAClosureMaxEntries),
		OACache:       NewClosureCache[uuid.UUID](cacheTTL, limits.OAClosureMaxEntries),
		AllowCache:    NewUserOpBitmapCache(cacheTTL, limits.AllowCacheMaxUsers, limits.AllowCacheMaxEntries),
		DenyCache:     NewUserOpBitmapCache(cacheTTL, limits.DenyCacheMaxUsers, limits.DenyCacheMaxEntries),
		UANodeClosure: NewClosureCache[uint32](10*time.Minute, limits.UANodeClosureMaxEntries),
		OANodeClosure: NewClosureCache[uint32](10*time.Minute, limits.OANodeClosureMaxEntries),
		OADescClosure: NewClosureCache[uint32](10*time.Minute, limits.OADescClosureMaxEntries),
		Decisions:     NewIndexedDecisionCache(60*time.Second, limits.DecisionCacheMaxEntries),
	}
}
