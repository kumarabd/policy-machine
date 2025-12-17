# Engine Implementation Details

This document provides in-depth coverage of the authorization engine's implementation, algorithms, and data structures.

## Table of Contents

- [Engine Structure](#engine-structure)
- [Snapshot Architecture](#snapshot-architecture)
- [Decision Algorithm](#decision-algorithm)
- [Closure Computation](#closure-computation)
- [Bitmap Operations](#bitmap-operations)
- [Index Management](#index-management)
- [Error Handling](#error-handling)

## Engine Structure

### Core Engine Type

The `Engine` struct (`pkg/engine/engine.go`) is the central component:

```go
type Engine struct {
    log      *logger.Handler
    metric   *metrics.Handler
    db       *postgres.Handler
    tenantID uuid.UUID

    cur atomic.Pointer[Snapshot]  // Current immutable snapshot

    // Closure caches (TTL-based)
    uaCache closureCache[uuid.UUID]      // User → UA closure
    oaCache closureCache[uuid.UUID]      // Object → OA closure
    uaNodeClosure closureCache[uint32]   // UA node → ancestors
    oaNodeClosure closureCache[uint32]   // OA node → ancestors
    oaDescClosure closureCache[uint32]   // OA node → descendants

    // Operation caches (TTL-based)
    allowCache *userOpBitmapCache        // User+Op → allowed OA bitmap
    denyCache  *userOpBitmapCache        // User+Op → denied OA bitmap

    // Decision cache (TTL-based, indexed)
    decisions *indexedDecisionCache

    refreshMu sync.Mutex  // Serializes refresh operations
    emitter ObligationEmitter
}
```

### Key Design Decisions

1. **Atomic Snapshot Pointer**: Enables lock-free reads
2. **Separate Caches**: Different TTLs for different cache types
3. **Indexed Decision Cache**: Fast invalidation by user/object/operation
4. **Mutex for Refresh**: Ensures only one update at a time

## Snapshot Architecture

### Snapshot Structure

The `Snapshot` type (`pkg/engine/snapshot.go`) represents an immutable policy graph:

```go
type Snapshot struct {
    TenantID uuid.UUID
    Version  int64  // Policy revision number
    LastSeq  int64  // Last applied change sequence

    // Dense indexing: UUID → uint32
    uaIndex map[uuid.UUID]uint32  // UA UUID → dense index
    uaByIdx []uuid.UUID           // Dense index → UA UUID
    oaIndex map[uuid.UUID]uint32  // OA UUID → dense index
    oaByIdx []uuid.UUID           // Dense index → OA UUID
    pcIndex map[uuid.UUID]uint32  // PC UUID → dense index
    pcByIdx []uuid.UUID           // Dense index → PC UUID

    // Policy class assignments
    uaToPCs map[uint32]*roaring.Bitmap  // UA idx → PC bitmaps
    oaToPCs map[uint32]*roaring.Bitmap  // OA idx → PC bitmaps

    // Assignment graphs
    userToUAs     map[uuid.UUID][]uint32  // User → direct UA children
    uaParents     map[uint32][]uint32     // UA → parent UAs
    uaChildren    map[uint32][]uint32     // UA → child UAs
    uaDirectUsers map[uint32][]uuid.UUID  // UA → direct user children

    objectToOAs     map[uuid.UUID][]uint32  // Object → direct OA children
    oaParents       map[uint32][]uint32     // OA → parent OAs
    oaChildren      map[uint32][]uint32     // OA → child OAs
    oaDirectObjects map[uint32][]uuid.UUID  // OA → direct object children

    // Permissions
    assoc map[uint32]map[string]*roaring.Bitmap  // UA idx → op → OA bitmap

    // Prohibitions
    userProhibits map[uuid.UUID]map[string]*roaring.Bitmap  // User → op → OA bitmap
    uaProhibits   map[uint32]map[string]*roaring.Bitmap    // UA idx → op → OA bitmap
}
```

### Why Dense Indices?

1. **Memory Efficiency**: `uint32` (4 bytes) vs `uuid.UUID` (16 bytes)
2. **Bitmap Compatibility**: Roaring Bitmaps work with `uint32` sets
3. **Cache Locality**: Dense arrays improve CPU cache performance
4. **Fast Lookups**: O(1) map lookups for UUID → index conversion

### Immutability Guarantees

- Snapshots are never modified after creation
- Updates create new snapshots via copy-on-write
- Old snapshots remain valid until garbage collected
- Readers can safely access snapshots without locks

## Decision Algorithm

### Main Decision Function

The `Decide` function (`pkg/engine/engine.go:168`) implements the NGAC decision algorithm:

```go
func (e *Engine) Decide(ctx context.Context, userID, objectID uuid.UUID, op string) (bool, error) {
    // 1. Get current snapshot (atomic read)
    s := e.Snapshot()
    if s == nil {
        if err := e.Refresh(ctx); err != nil {
            return false, err
        }
        s = e.Snapshot()
    }

    // 2. Check decision cache
    if allowed, ok := e.decisions.Get(userID, objectID, op, s.Version); ok {
        return allowed, nil
    }

    // 3. Compute closures
    uaClosure := e.userUAClosure(s, userID)
    oaClosure := e.objectOAClosure(s, objectID)

    // 4. Policy class intersection
    userPCs := computeUserPCs(s, uaClosure)
    objPCs := computeObjectPCs(s, oaClosure)
    userPCs.And(objPCs)
    if userPCs.IsEmpty() {
        e.decisions.Put(userID, objectID, op, false, s.Version)
        return false, nil
    }

    // 5. Compute allow/deny
    allowedBmp := e.allowedFor(s, userID, op, uaClosure)
    deniedBmp := e.deniedFor(s, userID, op, uaClosure)

    // 6. Final decision: (allowed - denied) ∩ oaClosure
    effective := allowedBmp.Clone()
    effective.AndNot(deniedBmp)
    effective.And(oaClosure)

    allowed := !effective.IsEmpty()
    e.decisions.Put(userID, objectID, op, allowed, s.Version)
    return allowed, nil
}
```

### Algorithm Steps Explained

#### Step 1: Snapshot Retrieval
- Atomic pointer read (no locking)
- If nil, trigger full refresh
- Ensures we always have a valid snapshot

#### Step 2: Cache Lookup
- Check indexed decision cache
- Validate against snapshot version
- Return immediately if cache hit

#### Step 3: Closure Computation
- **UA Closure**: All UAs reachable from user (via assignments)
- **OA Closure**: All OAs reachable from object (via assignments)
- Both use cached closures when available

#### Step 4: Policy Class Check
- Compute policy classes for user (via UA closure)
- Compute policy classes for object (via OA closure)
- Intersection must be non-empty (users and objects must share at least one PC)
- Early return if no shared PC (deny)

#### Step 5: Allow/Deny Computation
- **Allowed**: Aggregate all associations granting the operation
- **Denied**: Aggregate all prohibitions blocking the operation
- Both expand OA targets to descendants (hierarchical permissions)

#### Step 6: Final Decision
- Formula: `(allowed - denied) ∩ oaClosure`
- If result is non-empty → ALLOW
- Otherwise → DENY

### Allow Computation

```go
func (e *Engine) allowedFor(s *Snapshot, user uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
    // Check cache
    if b, ok := e.allowCache.Get(user, op); ok {
        return b
    }

    // Aggregate associations
    allowed := roaring.New()
    it := uaClosure.Iterator()
    for it.HasNext() {
        ua := it.Next()
        targets := s.assoc[ua][op]  // OA targets for this UA+op
        if targets != nil {
            it := targets.Iterator()
            for it.HasNext() {
                t := it.Next()
                // Expand to descendants (hierarchical permissions)
                allowed.Or(e.oaNodeDescendants(s, t))
            }
        }
    }

    e.allowCache.Put(user, op, allowed)
    return allowed
}
```

**Key Points:**
- Iterates over all UAs in user's closure
- For each UA, finds associations for the operation
- Expands OA targets to descendants (hierarchical)
- Unions all results into single bitmap

### Deny Computation

```go
func (e *Engine) deniedFor(s *Snapshot, user uuid.UUID, op string, uaClosure *roaring.Bitmap) *roaring.Bitmap {
    // Check cache
    if b, ok := e.denyCache.Get(user, op); ok {
        return b
    }

    denied := roaring.New()

    // UA-level prohibitions
    it := uaClosure.Iterator()
    for it.HasNext() {
        ua := it.Next()
        opMap := s.uaProhibits[ua]
        if opMap == nil {
            continue
        }
        targets := opMap[op]
        if targets == nil {
            continue
        }
        tit := targets.Iterator()
        for tit.HasNext() {
            t := tit.Next()
            denied.Or(e.oaNodeDescendants(s, t))
        }
    }

    // User-level prohibitions
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
```

**Key Points:**
- Checks both UA-level and user-level prohibitions
- Expands OA targets to descendants
- Unions all prohibitions into single bitmap

## Closure Computation

### User UA Closure

The user UA closure is the set of all UAs reachable from a user via assignment edges:

```go
func (e *Engine) userUAClosure(s *Snapshot, userID uuid.UUID) *roaring.Bitmap {
    // Check cache
    if bmp, ok := e.uaCache.Get(userID); ok {
        return bmp
    }

    // Start with user's direct UA assignments
    out := roaring.New()
    for _, ua := range s.userToUAs[userID] {
        // Get all ancestors of this UA (including itself)
        out.Or(e.uaNodeAllParents(s, ua))
    }

    e.uaCache.Put(userID, out)
    return out
}
```

**Algorithm:**
1. Get user's direct UA assignments
2. For each UA, compute its ancestor closure (including itself)
3. Union all results

### Object OA Closure

Similar to user UA closure, but for objects:

```go
func (e *Engine) objectOAClosure(s *Snapshot, objectID uuid.UUID) *roaring.Bitmap {
    // Check cache
    if bmp, ok := e.oaCache.Get(objectID); ok {
        return bmp
    }

    // Start with object's direct OA assignments
    out := roaring.New()
    for _, oa := range s.objectToOAs[objectID] {
        // Get all ancestors of this OA (including itself)
        out.Or(e.oaNodeAllParents(s, oa))
    }

    e.oaCache.Put(objectID, out)
    return out
}
```

### Node Ancestor Closure

Computes all ancestors of a UA/OA node (including itself):

```go
func (e *Engine) uaNodeAllParents(s *Snapshot, ua uint32) *roaring.Bitmap {
    // Check cache
    if b, ok := e.uaNodeClosure.Get(ua); ok {
        return b
    }

    // BFS traversal
    out := roaring.New()
    queue := []uint32{ua}
    out.Add(ua)

    for i := 0; i < len(queue); i++ {
        cur := queue[i]
        for _, p := range s.uaParents[cur] {
            if out.CheckedAdd(p) {
                queue = append(queue, p)
            }
        }
    }

    e.uaNodeClosure.Put(ua, out)
    return out
}
```

**Algorithm:**
- BFS traversal starting from the node
- Follows parent edges upward
- Includes the starting node itself
- Caches result for reuse

### Node Descendant Closure

Computes all descendants of an OA node (for hierarchical permissions):

```go
func (e *Engine) oaNodeDescendants(s *Snapshot, oa uint32) *roaring.Bitmap {
    // Check cache
    if b, ok := e.oaDescClosure.Get(oa); ok {
        return b
    }

    // BFS traversal downward
    out := roaring.New()
    queue := []uint32{oa}
    out.Add(oa)

    for i := 0; i < len(queue); i++ {
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
```

**Use Case:**
- When an association grants permission on an OA
- We expand to all descendant OAs (hierarchical permissions)
- Objects in child OAs inherit permissions from parent OAs

## Bitmap Operations

### Why Roaring Bitmaps?

1. **Memory Efficient**: Compressed representation for sparse sets
2. **Fast Operations**: O(n) intersections, unions, differences
3. **Scalable**: Handles sets with millions of elements
4. **Immutable-Friendly**: Clone operations are efficient

### Common Operations

```go
// Create new bitmap
bmp := roaring.New()

// Add elements
bmp.Add(42)
bmp.Add(100)

// Set operations
result := bmp1.Clone()
result.Or(bmp2)        // Union
result.And(bmp2)       // Intersection
result.AndNot(bmp2)    // Difference

// Check emptiness
if bmp.IsEmpty() {
    // ...
}

// Iterate
it := bmp.Iterator()
for it.HasNext() {
    val := it.Next()
    // ...
}
```

### Performance Characteristics

- **Intersection**: O(min(n1, n2)) where n1, n2 are set sizes
- **Union**: O(n1 + n2)
- **Difference**: O(n1)
- **Membership**: O(log n) with binary search

## Index Management

### Dense Index Creation

During snapshot loading (`pkg/engine/load.go`):

```go
// Load UAs and create dense indices
var uas []postgres.UserAttribute
db.Where("tenant_id = ?", tenantID).Find(&uas)

for _, ua := range uas {
    idx := uint32(len(s.uaByIdx))
    s.uaByIdx = append(s.uaByIdx, ua.ID)
    s.uaIndex[ua.ID] = idx
}
```

**Benefits:**
- Sequential indices (0, 1, 2, ...)
- Efficient bitmap operations
- Fast UUID → index lookups

### Index Expansion

During incremental updates, new entities may be added:

```go
func ensureUAIdx(s *Snapshot, uaID uuid.UUID) (uint32, bool) {
    if idx, ok := s.uaIndex[uaID]; ok {
        return idx, true
    }
    // Append-only expansion
    idx := uint32(len(s.uaByIdx))
    s.uaByIdx = append(s.uaByIdx, uaID)
    s.uaIndex[uaID] = idx
    return idx, true
}
```

**Note:** Indices are never compacted (append-only). This simplifies copy-on-write.

## Error Handling

### Snapshot Loading Errors

```go
func LoadSnapshot(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*Snapshot, error) {
    // If any database query fails, return error
    // Caller should handle (e.g., retry, fallback)
}
```

### Decision Errors

```go
func (e *Engine) Decide(ctx context.Context, ...) (bool, error) {
    // Errors can occur from:
    // - Database queries (if refresh needed)
    // - Invalid snapshot state
    // - Context cancellation
    
    // Best practice: Return error, don't default to deny
    // Let caller decide on error handling strategy
}
```

### Refresh Errors

```go
func (e *Engine) RefreshIncremental(ctx context.Context) error {
    // On any error, fallback to full refresh
    // This ensures correctness over performance
    if err != nil {
        return e.Refresh(ctx)
    }
}
```

### Graceful Degradation

- **Cache misses**: Compute on-the-fly (slower but correct)
- **Refresh failures**: Retry with exponential backoff
- **Database errors**: Log and continue serving cached decisions
- **Invalid changes**: Fallback to full refresh

## Performance Optimizations

### 1. Early Returns

- Policy class check fails → immediate deny
- Cache hits → immediate return
- Empty closures → early termination

### 2. Lazy Computation

- Closures computed only when needed
- Bitmaps cloned only when mutated
- Caches populated on-demand

### 3. Bitmap Sharing

- Immutable bitmaps can be shared
- Clone only when modifying
- Reduces memory allocations

### 4. Iterator Efficiency

- Roaring Bitmap iterators are efficient
- Avoid materializing full sets
- Stream processing for large sets

## Next Steps

- See [CACHING.md](CACHING.md) for caching implementation details
- See [INCREMENTAL_REFRESH.md](INCREMENTAL_REFRESH.md) for update mechanisms
- See [ARCHITECTURE.md](ARCHITECTURE.md) for system overview

