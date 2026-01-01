# Caching Strategy

This document details the multi-tier caching system used in Policy Machine to achieve microsecond-level authorization decisions.

## Table of Contents

- [Cache Architecture](#cache-architecture)
- [Cache Types](#cache-types)
- [Cache Invalidation](#cache-invalidation)
- [TTL Configuration](#ttl-configuration)
- [Performance Impact](#performance-impact)
- [Memory Management](#memory-management)

## Cache Architecture

Policy Machine uses a **multi-tier caching system** with different cache types optimized for different access patterns:

```
┌─────────────────────────────────────────────────────────┐
│              Decision Cache (60s TTL)                   │
│  Indexed by: (subject, object, operation)                  │
│  Fastest path: 1-10μs                                   │
└─────────────────────────────────────────────────────────┘
                      │ (miss)
                      ▼
┌─────────────────────────────────────────────────────────┐
│        Operation Caches (2min TTL)                      │
│  • Allow Cache: (subject, operation) → OA bitmap            │
│  • Deny Cache: (subject, operation) → OA bitmap             │
└─────────────────────────────────────────────────────────┘
                      │ (miss)
                      ▼
┌─────────────────────────────────────────────────────────┐
│        Closure Caches (2-10min TTL)                     │
│  • UA Closure: subject → UA bitmap                         │
│  • OA Closure: object → OA bitmap                      │
│  • Node Closures: UA/OA node → ancestors/descendants   │
└─────────────────────────────────────────────────────────┘
                      │ (miss)
                      ▼
┌─────────────────────────────────────────────────────────┐
│              Snapshot (Immutable)                        │
│  Policy graph in memory                                 │
└─────────────────────────────────────────────────────────┘
```

## Cache Types

### 1. Decision Cache

**Purpose**: Cache final authorization decisions for fastest response.

**Implementation**: `indexedDecisionCache` (`pkg/engine/decision_cache.go`)

**Key Features**:
- **Indexed by subject, object, operation**
- **TTL**: 60 seconds
- **Revision validation**: Invalidated on policy changes
- **Fast invalidation**: Can delete by subject, object, or (subject, operation)

**Structure**:
```go
type indexedDecisionCache struct {
    ttl time.Duration
    
    entries  map[decisionKey]decisionEntry
    bySubject   map[uuid.UUID]keySet      // Index for subject-based invalidation
    byObject map[uuid.UUID]keySet      // Index for object-based invalidation
    bySubjectOp map[uuid.UUID]map[string]keySet  // Index for (subject, op) invalidation
}
```

**Access Pattern**:
```go
// Get decision
allowed, ok := cache.Get(subjectID, objectID, op, snapshotVersion)

// Put decision
cache.Put(subjectID, objectID, op, allowed, snapshotVersion)
```

**Performance**:
- **Hit**: O(1) map lookup → 1-10μs
- **Miss**: Triggers computation → 50-200μs

### 2. Closure Caches

#### Subject UA Closure Cache

**Purpose**: Cache the set of all UAs reachable from a subject.

**Implementation**: `closureCache[uuid.UUID]` (`pkg/engine/closure_cache.go`)

**TTL**: 2 minutes

**Key**: Subject UUID

**Value**: Roaring Bitmap of UA indices

**Usage**:
```go
uaClosure := e.subjectUAClosure(snapshot, subjectID)
// Returns cached bitmap if available, otherwise computes and caches
```

#### Object OA Closure Cache

**Purpose**: Cache the set of all OAs reachable from an object.

**Implementation**: `closureCache[uuid.UUID]`

**TTL**: 2 minutes

**Key**: Object UUID

**Value**: Roaring Bitmap of OA indices

**Usage**:
```go
oaClosure := e.objectOAClosure(snapshot, objectID)
```

#### Node Closure Caches

**Purpose**: Cache ancestor/descendant closures for UA/OA nodes.

**Implementation**: `closureCache[uint32]`

**TTL**: 10 minutes (longer because node structure changes less frequently)

**Types**:
- `uaNodeClosure`: UA node → all ancestors (including itself)
- `oaNodeClosure`: OA node → all ancestors (including itself)
- `oaDescClosure`: OA node → all descendants (including itself)

**Usage**:
```go
ancestors := e.uaNodeAllParents(snapshot, uaIdx)
descendants := e.oaNodeDescendants(snapshot, oaIdx)
```

### 3. Operation Caches

#### Allow Cache

**Purpose**: Cache the set of OAs a subject can access for an operation.

**Implementation**: `subjectOpBitmapCache` (`pkg/engine/subjectop_cache.go`)

**TTL**: 2 minutes

**Key**: (Subject UUID, Operation string)

**Value**: Roaring Bitmap of OA indices

**Computation**: Aggregates all associations granting the operation, expanding to OA descendants.

**Usage**:
```go
allowed := e.allowedFor(snapshot, subjectID, op, uaClosure)
```

#### Deny Cache

**Purpose**: Cache the set of OAs a subject is prohibited from accessing for an operation.

**Implementation**: `subjectOpBitmapCache`

**TTL**: 2 minutes

**Key**: (Subject UUID, Operation string)

**Value**: Roaring Bitmap of OA indices

**Computation**: Aggregates all prohibitions blocking the operation, expanding to OA descendants.

**Usage**:
```go
denied := e.deniedFor(snapshot, subjectID, op, uaClosure)
```

## Cache Invalidation

### Invalidation Strategy

Policy Machine uses **granular cache invalidation** to minimize cache misses while ensuring correctness:

1. **Compute affected entities**: Determine which subjects, objects, and operations are affected by a policy change
2. **Invalidate selectively**: Only delete cache entries that are actually affected
3. **Preserve unaffected entries**: Keep cache entries that remain valid

### Invalidation Types

#### 1. Subject UA Closure Invalidation

**Triggered by**:
- Subject → UA assignment changes
- UA → UA hierarchy changes (affects all subjects in subtree)

**Action**:
- Delete subject's UA closure cache entry
- Delete subject's allow/deny cache entries (all operations)
- Delete subject's decision cache entries

**Example**:
```go
// Subject assigned to new UA
inv.subjectsUAClosure[subjectID] = struct{}{}
// Later in applyInvalidations:
e.uaCache.Delete(subjectID)
e.allowCache.DeleteSubject(subjectID)
e.denyCache.DeleteSubject(subjectID)
e.decisions.DeleteSubject(subjectID)
```

#### 2. Object OA Closure Invalidation

**Triggered by**:
- Object → OA assignment changes
- OA → OA hierarchy changes (affects all objects in subtree)

**Action**:
- Delete object's OA closure cache entry
- Delete object's decision cache entries

**Example**:
```go
// Object assigned to new OA
inv.objectsOAClosure[objectID] = struct{}{}
// Later:
e.oaCache.Delete(objectID)
e.decisions.DeleteObject(objectID)
```

#### 3. Operation-Specific Invalidation

**Triggered by**:
- Association changes (affects allow cache)
- Prohibition changes (affects deny cache)

**Action**:
- Delete specific (subject, operation) cache entries
- Delete affected decision cache entries

**Example**:
```go
// Association added for UA
for subject in UA.subtree:
    inv.subjectAllowOp[subject][op] = struct{}{}
// Later:
e.allowCache.DeleteSubjectOp(subject, op)
e.decisions.DeleteSubjectOp(subject, op)
```

#### 4. Node Closure Invalidation

**Triggered by**:
- UA → UA hierarchy changes
- OA → OA hierarchy changes

**Action**:
- Delete affected node closure cache entries
- Cascades to subject/object closures that depend on these nodes

**Example**:
```go
// UA hierarchy changed
for ua in affected_subtree:
    inv.uaNodeClosures[ua] = struct{}{}
// Later:
e.uaNodeClosure.Delete(ua)
```

### Invalidation Computation

The `computeInvalidations` function (`pkg/engine/invalidate.go`) analyzes policy changes to determine affected entities:

```go
func computeInvalidations(inv *invalidation, s *Snapshot, ch postgres.PolicyChange) {
    switch ch.Kind {
    case "ASSIGNMENT_EDGE":
        // Analyze edge type and determine affected subjects/objects
        if childType == USER && parentType == UA {
            inv.subjectsUAClosure[childID] = struct{}{}
        }
        // ... more cases
        
    case "ASSOC_OP":
        // Association change affects allow cache
        for subject in UA.subtree:
            inv.addAllow(subject, operation)
            
    case "PROHIB_OP":
        // Prohibition change affects deny cache
        for subject in affected_subjects:
            inv.addDeny(subject, operation)
    }
}
```

### Invalidation Application

The `applyInvalidations` function (`pkg/engine/engine.go:225`) applies computed invalidations:

```go
func (e *Engine) applyInvalidations(inv *invalidation) {
    // 1. Invalidate node closures
    for uaIdx := range inv.uaNodeClosures {
        e.uaNodeClosure.Delete(uaIdx)
    }
    
    // 2. Invalidate subject closures and derived caches
    for u := range inv.subjectsUAClosure {
        e.uaCache.Delete(u)
        e.allowCache.DeleteSubject(u)
        e.denyCache.DeleteSubject(u)
        e.decisions.DeleteSubject(u)
    }
    
    // 3. Invalidate object closures
    for o := range inv.objectsOAClosure {
        e.oaCache.Delete(o)
        e.decisions.DeleteObject(o)
    }
    
    // 4. Invalidate operation-specific caches
    for u, ops := range inv.subjectAllowOp {
        for op := range ops {
            e.allowCache.DeleteSubjectOp(u, op)
            e.decisions.DeleteSubjectOp(u, op)
        }
    }
    // ... similar for deny
}
```

## TTL Configuration

### Current TTLs

Configured in `engine.New()` (`pkg/engine/engine.go:48`):

```go
e := &Engine{
    uaCache:       newClosureCache[uuid.UUID](2 * time.Minute),
    oaCache:       newClosureCache[uuid.UUID](2 * time.Minute),
    allowCache:    newSubjectOpBitmapCache(2 * time.Minute),
    denyCache:     newSubjectOpBitmapCache(2 * time.Minute),
    uaNodeClosure: newClosureCache[uint32](10 * time.Minute),
    oaNodeClosure: newClosureCache[uint32](10 * time.Minute),
    oaDescClosure: newClosureCache[uint32](10 * time.Minute),
    decisions:     newIndexedDecisionCache(60 * time.Second),
}
```

### TTL Rationale

1. **Decision Cache (60s)**: Short TTL because decisions are most likely to change
2. **Closure Caches (2min)**: Medium TTL balances freshness vs. computation cost
3. **Node Closures (10min)**: Long TTL because graph structure changes infrequently

### TTL Trade-offs

**Longer TTLs**:
- ✅ Higher cache hit rate
- ✅ Lower computation cost
- ❌ Stale data risk (mitigated by invalidation)
- ❌ Higher memory usage

**Shorter TTLs**:
- ✅ Fresher data
- ✅ Lower memory usage
- ❌ More cache misses
- ❌ Higher computation cost

### TTL Adjustment Guidelines

Adjust TTLs based on your workload:

- **High policy change frequency**: Shorter TTLs (30s-1min)
- **Low policy change frequency**: Longer TTLs (5-10min)
- **Memory-constrained**: Shorter TTLs
- **CPU-constrained**: Longer TTLs (more cache hits)

## Performance Impact

### Cache Hit Rates

Typical cache hit rates in production:

- **Decision Cache**: 80-95% (most requests are repeated)
- **Closure Caches**: 60-80% (subjects/objects accessed multiple times)
- **Node Closures**: 90-99% (graph structure is stable)

### Latency Impact

| Cache State | Latency | Description |
|------------|---------|-------------|
| **All hits** | 1-10μs | Decision cache hit |
| **Closure hits** | 50-200μs | Compute decision, reuse closures |
| **Node hits** | 200-500μs | Compute closures, reuse node closures |
| **All misses** | 1-5ms | Full computation + database queries |

### Throughput Impact

- **With caching**: 10K-100K+ decisions/sec
- **Without caching**: 1K-5K decisions/sec (10-100x slower)

## Memory Management

### Memory Footprint

Per cache type (approximate, 10K entities):

| Cache Type | Size | Notes |
|------------|------|-------|
| Decision Cache | 1-5 MB | Depends on unique (subject, object, op) combinations |
| Closure Caches | 500 KB - 2 MB | Depends on graph density |
| Node Closures | 100-500 KB | Relatively stable |
| Operation Caches | 1-3 MB | Depends on operation diversity |
| **Total** | **2.5-10 MB** | Per tenant |

### Memory Optimization

1. **Lazy Expiration**: Entries expire on access (not proactively)
2. **Immutable Bitmaps**: Can be shared until mutated
3. **Roaring Compression**: Efficient storage for sparse sets
4. **Indexed Cleanup**: Fast deletion of related entries

### Cache Eviction

Currently, caches use **TTL-based expiration** with lazy cleanup:

```go
func (c *closureCache[K]) Get(k K) (*roaring.Bitmap, bool) {
    now := time.Now()
    e, ok := c.m[k]
    if !ok || now.After(e.expires) {
        return nil, false  // Expired or missing
    }
    return e.bmp, true
}
```

**Future Enhancements**:
- LRU eviction for memory pressure
- Size-based limits
- Periodic cleanup goroutine

## Monitoring Cache Performance

### Metrics to Track

1. **Cache Hit Rates**: Per cache type
2. **Cache Sizes**: Memory usage per cache
3. **Eviction Rates**: How often entries expire
4. **Computation Times**: Cost of cache misses

### Example Metrics

```go
// Add to metrics handler
cacheHits := prometheus.NewCounterVec(...)
cacheMisses := prometheus.NewCounterVec(...)
cacheSize := prometheus.NewGaugeVec(...)
```

### Debugging Cache Issues

1. **Low hit rates**: Check TTLs, invalidation frequency
2. **High memory**: Check cache sizes, consider shorter TTLs
3. **Stale data**: Verify invalidation logic, check revision validation
4. **Slow decisions**: Check cache miss rates, optimize computation

## Best Practices

1. **Monitor hit rates**: Aim for >80% decision cache hit rate
2. **Tune TTLs**: Balance freshness vs. performance
3. **Validate revisions**: Ensure cache entries match snapshot version
4. **Granular invalidation**: Only invalidate what's necessary
5. **Warmup after refresh**: Pre-compute closures for frequently accessed entities

## Next Steps

- See [ENGINE.md](ENGINE.md) for engine implementation details
- See [INCREMENTAL_REFRESH.md](INCREMENTAL_REFRESH.md) for update mechanisms
- See [ARCHITECTURE.md](ARCHITECTURE.md) for system overview

