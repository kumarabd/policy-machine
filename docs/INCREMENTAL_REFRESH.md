# Incremental Refresh and Change Tracking

This document explains how Policy Machine handles policy updates incrementally using a change log, copy-on-write snapshots, and granular cache invalidation.

## Table of Contents

- [Overview](#overview)
- [Change Log Design](#change-log-design)
- [Incremental Refresh Algorithm](#incremental-refresh-algorithm)
- [Copy-on-Write Updates](#copy-on-write-updates)
- [Cache Invalidation](#cache-invalidation)
- [Gap Detection and Recovery](#gap-detection-and-recovery)
- [Performance Characteristics](#performance-characteristics)

## Overview

Policy Machine supports **incremental policy updates** to avoid expensive full snapshot rebuilds. The system uses:

1. **Change Log**: Append-only log of all policy changes
2. **Copy-on-Write**: Efficient snapshot updates without full rebuilds
3. **Granular Invalidation**: Only invalidate affected cache entries
4. **Atomic Swaps**: New snapshots replace old ones atomically

### Benefits

- **Fast Updates**: 10-100x faster than full refresh
- **Low Latency**: Updates don't block authorization decisions
- **Memory Efficient**: Only clone what changes
- **Correctness**: Fallback to full refresh on errors

## Change Log Design

### Policy Changes Table

The `policy_changes` table (`pkg/postgres/changes.go`) stores all policy modifications:

```go
type PolicyChange struct {
    Seq      int64     `gorm:"primaryKey;autoIncrement"`
    TenantID uuid.UUID `gorm:"type:uuid;not null;index"`
    Revision int64     `gorm:"not null;index"`
    
    Kind    string    `gorm:"type:text;not null"`  // ASSIGNMENT_EDGE, ASSOC_OP, PROHIB_OP
    Op      ChangeOp  `gorm:"type:text;not null"`  // ADD, REMOVE, UPDATE
    Payload datatypes.JSON `gorm:"type:jsonb;not null"`
    
    CreatedAt time.Time `gorm:"autoCreateTime"`
}
```

### Change Kinds

#### 1. ASSIGNMENT_EDGE

Represents assignment edge changes (subject→UA, UA→UA, object→OA, etc.):

```json
{
  "child_type": "USER",
  "child_id": "550e8400-e29b-41d4-a716-446655440000",
  "parent_type": "UA",
  "parent_id": "660e8400-e29b-41d4-a716-446655440001"
}
```

**Operations**: `ADD`, `REMOVE`

#### 2. ASSOC_OP

Represents association operation changes:

```json
{
  "ua_id": "550e8400-e29b-41d4-a716-446655440000",
  "oa_id": "660e8400-e29b-41d4-a716-446655440001",
  "op": "read"
}
```

**Operations**: `ADD`, `REMOVE`

#### 3. PROHIB_OP

Represents prohibition operation changes:

```json
{
  "subject_type": "USER",
  "subject_id": "550e8400-e29b-41d4-a716-446655440000",
  "oa_id": "660e8400-e29b-41d4-a716-446655440001",
  "op": "write"
}
```

**Operations**: `ADD`, `REMOVE`

### Sequence Numbers

- **Monotonic**: Sequence numbers always increase
- **Continuous**: No gaps (gaps trigger full refresh)
- **Ordered**: Changes applied in sequence order
- **Per-tenant**: Each tenant has its own sequence

### Revision Numbers

- **Global per tenant**: Single revision counter in `policy_revisions` table
- **Incremented on change**: Every change increments revision
- **Used for validation**: Cache entries validated against revision

## Incremental Refresh Algorithm

### Main Refresh Function

The `RefreshIncremental` function (`pkg/engine/refresh.go:11`) implements incremental updates:

```go
func (e *Engine) RefreshIncremental(ctx context.Context) error {
    e.refreshMu.Lock()
    defer e.refreshMu.Unlock()

    s := e.Snapshot()
    if s == nil {
        return e.Refresh(ctx)  // Bootstrap: full refresh
    }

    // 1. Check current revision
    var rev postgres.PolicyRevision
    if err := e.db.H.WithContext(ctx).First(&rev, "tenant_id = ?", e.tenantID).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil  // No policy data
        }
        return err
    }

    if rev.Revision <= s.Version {
        return nil  // Already up to date
    }

    // 2. Fetch changes since last sequence
    var changes []postgres.PolicyChange
    if err := e.db.H.WithContext(ctx).
        Where("tenant_id = ? AND seq > ?", e.tenantID, s.LastSeq).
        Order("seq ASC").
        Find(&changes).Error; err != nil {
        return err
    }

    // 3. Gap detection
    if len(changes) == 0 {
        return e.Refresh(ctx)  // Revision advanced but no changes -> gap
    }
    if !validateSeqContinuity(s.LastSeq, changes) {
        return e.Refresh(ctx)  // Gap detected -> full refresh
    }

    // 4. Copy-on-write update
    inv := newInvalidation()
    next := cloneSnapshotShallow(s)
    lastRevision := s.Version
    
    for _, ch := range changes {
        // Revision monotonic check
        if ch.Revision < lastRevision {
            return e.Refresh(ctx)  // Revision went backwards -> full refresh
        }
        lastRevision = ch.Revision

        // Compute invalidations
        computeInvalidations(inv, s, ch)
        
        // Apply change
        if err := applyChange(next, ch); err != nil {
            return e.Refresh(ctx)  // Error -> full refresh
        }
        
        next.LastSeq = ch.Seq
        next.Version = ch.Revision
    }

    // 5. Atomic swap
    e.cur.Store(next)
    
    // 6. Compute additional invalidations
    e.computeSubjectOpInvalidationsFromOADescChanges(s, inv)
    
    // 7. Apply invalidations
    e.applyInvalidations(inv)
    e.uaCache.Clear()
    e.oaCache.Clear()

    // 8. Warmup (optional, async)
    go e.Warmup(next, inv)

    return nil
}
```

### Algorithm Steps

#### Step 1: Revision Check

Check if policy has changed since last snapshot:

```go
if rev.Revision <= s.Version {
    return nil  // No changes
}
```

**Early return**: If no changes, skip refresh entirely.

#### Step 2: Fetch Changes

Retrieve all changes since last applied sequence:

```go
db.Where("tenant_id = ? AND seq > ?", tenantID, lastSeq)
  .Order("seq ASC")
  .Find(&changes)
```

**Index used**: `(tenant_id, seq)` for fast ordered retrieval.

#### Step 3: Gap Detection

Validate sequence continuity:

```go
func validateSeqContinuity(lastSeq int64, changes []postgres.PolicyChange) bool {
    if len(changes) == 0 {
        return true
    }
    if lastSeq != 0 && changes[0].Seq != lastSeq+1 {
        return false  // Gap at start
    }
    for i := 1; i < len(changes); i++ {
        if changes[i].Seq != changes[i-1].Seq+1 {
            return false  // Gap in middle
        }
    }
    return true
}
```

**Gap scenarios**:
- Changes deleted (retention policy)
- Replica lag
- Manual database modifications

**Fallback**: Full refresh on gap detection.

#### Step 4: Copy-on-Write Update

For each change:

1. **Compute invalidations**: Determine affected cache entries
2. **Apply change**: Update snapshot copy-on-write
3. **Update metadata**: Set sequence and revision

#### Step 5: Atomic Swap

Replace old snapshot with new one:

```go
e.cur.Store(next)
```

**Thread-safe**: All readers see new snapshot immediately.

#### Step 6: Cache Invalidation

Invalidate affected cache entries (see [Cache Invalidation](#cache-invalidation)).

#### Step 7: Warmup (Optional)

Pre-compute closures for frequently accessed entities:

```go
go e.Warmup(next, inv)
```

**Async**: Doesn't block refresh completion.

## Copy-on-Write Updates

### Shallow Clone

Create a shallow copy of the snapshot:

```go
func cloneSnapshotShallow(s *Snapshot) *Snapshot {
    cp := *s  // Copy struct
    return &cp
}
```

**Note**: Maps still alias! We clone maps only when mutating.

### Map Cloning

Clone maps only when we need to modify them:

```go
func cowMapSlice[M ~map[uuid.UUID][]uint32](m M) M {
    cp := make(M, len(m))
    for k, v := range m {
        cp[k] = v  // Slice aliases (immutable in our usage)
    }
    return cp
}
```

**Efficiency**: Only clone maps we actually modify.

### Bitmap Cloning

Roaring Bitmaps are cloned when mutated:

```go
b := s.assoc[uaIdx][op]
if b == nil {
    b = roaring.New()
} else {
    b = b.Clone()  // Clone before mutation
}
b.Add(oaIdx)
s.assoc[uaIdx][op] = b
```

**Immutable bitmaps**: Can be shared until mutated.

### Change Application

The `applyChange` function (`pkg/engine/apply.go:12`) applies changes:

```go
func applyChange(s *Snapshot, ch postgres.PolicyChange) error {
    switch ch.Kind {
    case "ASSIGNMENT_EDGE":
        return applyAssignmentEdge(s, ch.Op, ...)
    case "ASSOC_OP":
        return applyAssocOp(s, ch.Op, ...)
    case "PROHIB_OP":
        return applyProhibOp(s, ch.Op, ...)
    }
}
```

**Error handling**: On any error, fallback to full refresh.

## Cache Invalidation

### Invalidation Computation

The `computeInvalidations` function (`pkg/engine/invalidate.go:53`) analyzes changes:

```go
func computeInvalidations(inv *invalidation, s *Snapshot, ch postgres.PolicyChange) {
    switch ch.Kind {
    case "ASSIGNMENT_EDGE":
        // Analyze edge type
        if childType == USER && parentType == UA {
            inv.subjectsUAClosure[childID] = struct{}{}
        }
        // ... more cases
        
    case "ASSOC_OP":
        // Association change affects allow cache
        uaIdx := s.uaIndex[uaID]
        for _, u := range s.SubjectsInUASubtree(uaIdx) {
            inv.addAllow(u, op)
        }
        
    case "PROHIB_OP":
        // Prohibition change affects deny cache
        if subjectType == USER {
            inv.addDeny(subjectID, op)
        } else {
            uaIdx := s.uaIndex[subjectID]
            for _, u := range s.SubjectsInUASubtree(uaIdx) {
                inv.addDeny(u, op)
            }
        }
    }
}
```

### Invalidation Structure

The `invalidation` struct tracks what to invalidate:

```go
type invalidation struct {
    subjectsUAClosure   map[uuid.UUID]struct{}  // Subjects whose UA closure changed
    objectsOAClosure map[uuid.UUID]struct{}   // Objects whose OA closure changed
    
    subjectAllowOp map[uuid.UUID]map[string]struct{}  // (subject, op) allow cache
    subjectDenyOp  map[uuid.UUID]map[string]struct{}  // (subject, op) deny cache
    
    uaNodeClosures map[uint32]struct{}  // UA node closures to invalidate
    oaNodeClosures map[uint32]struct{}  // OA node closures to invalidate
    
    subjectsDecisionsOnly   map[uuid.UUID]struct{}  // Only decisions (not closures)
    objectsDecisionsOnly map[uuid.UUID]struct{}  // Only decisions (not closures)
    oaDescClosures       map[uint32]struct{}      // OA descendant closures
}
```

### Invalidation Application

The `applyInvalidations` function (`pkg/engine/engine.go:225`) applies invalidations:

```go
func (e *Engine) applyInvalidations(inv *invalidation) {
    // 1. Node closures
    for uaIdx := range inv.uaNodeClosures {
        e.uaNodeClosure.Delete(uaIdx)
    }
    
    // 2. Subject closures and derived caches
    for u := range inv.subjectsUAClosure {
        e.uaCache.Delete(u)
        e.allowCache.DeleteSubject(u)
        e.denyCache.DeleteSubject(u)
        e.decisions.DeleteSubject(u)
    }
    
    // 3. Object closures
    for o := range inv.objectsOAClosure {
        e.oaCache.Delete(o)
        e.decisions.DeleteObject(o)
    }
    
    // 4. Operation-specific
    for u, ops := range inv.subjectAllowOp {
        for op := range ops {
            e.allowCache.DeleteSubjectOp(u, op)
            e.decisions.DeleteSubjectOp(u, op)
        }
    }
    // ... similar for deny
}
```

## Gap Detection and Recovery

### Gap Scenarios

1. **Changes Deleted**: Retention policy removed old changes
2. **Replica Lag**: Reading from stale replica
3. **Manual Modifications**: Direct database changes bypassed change log
4. **Sequence Reset**: Sequence numbers reset (shouldn't happen)

### Detection

Gaps are detected in two ways:

1. **Empty Changes**: Revision advanced but no changes found
2. **Sequence Discontinuity**: Sequence numbers not continuous

```go
// Empty changes
if len(changes) == 0 {
    return e.Refresh(ctx)  // Gap -> full refresh
}

// Sequence discontinuity
if !validateSeqContinuity(s.LastSeq, changes) {
    return e.Refresh(ctx)  // Gap -> full refresh
}
```

### Recovery

On gap detection, fallback to full refresh:

```go
return e.Refresh(ctx)  // Full snapshot rebuild
```

**Trade-off**: Correctness over performance.

## Performance Characteristics

### Incremental Refresh Performance

| Scenario | Latency | Description |
|----------|---------|-------------|
| **No changes** | <1ms | Early return on revision check |
| **Small update** (1-10 changes) | 1-5ms | Fast copy-on-write |
| **Medium update** (10-100 changes) | 5-20ms | Multiple changes applied |
| **Large update** (100+ changes) | 20-100ms | Many changes, more invalidation |
| **Full refresh** | 10-100ms | Complete snapshot rebuild |

### Memory Impact

- **Copy-on-write**: Only clone mutated maps/bitmaps
- **Memory overhead**: ~10-50% of snapshot size (depends on change scope)
- **Garbage collection**: Old snapshots GC'd when no longer referenced

### Throughput Impact

- **Incremental refresh**: Doesn't block authorization decisions
- **Full refresh**: Blocks briefly (10-100ms)
- **Concurrent requests**: Continue using old snapshot during refresh

## Best Practices

### 1. Change Log Maintenance

- **Retention**: Keep changes for at least 7 days
- **Archival**: Archive old changes to separate table
- **Monitoring**: Alert on gap detection frequency

### 2. Refresh Frequency

- **On-demand**: Trigger refresh after policy changes
- **Periodic**: Refresh every 1-5 minutes as backup
- **Event-driven**: Refresh on revision change notifications

### 3. Error Handling

- **Retry logic**: Retry failed refreshes with exponential backoff
- **Fallback**: Always fallback to full refresh on errors
- **Logging**: Log all refresh operations for debugging

### 4. Monitoring

Track:
- Refresh frequency and latency
- Gap detection rate
- Full refresh fallback rate
- Cache invalidation impact

## Implementation Details

### Change Logging

When policy changes occur, they should be logged to `policy_changes`:

```go
// Example: Add assignment edge
change := postgres.PolicyChange{
    TenantID: tenantID,
    Revision: nextRevision,
    Kind:     "ASSIGNMENT_EDGE",
    Op:       postgres.OpAdd,
    Payload:  json.Marshal(payload),
}
db.Create(&change)

// Increment revision
db.Model(&postgres.PolicyRevision{}).
   Where("tenant_id = ?", tenantID).
   Update("revision", gorm.Expr("revision + 1"))
```

### Refresh Triggering

Refresh can be triggered:

1. **Manually**: `engine.RefreshIncremental(ctx)`
2. **Periodically**: Background goroutine
3. **On revision change**: Database trigger or notification

### Warmup Strategy

After refresh, warmup frequently accessed entities:

```go
func (e *Engine) Warmup(s *Snapshot, inv *invalidation) {
    // Warm subject closures
    for u := range inv.subjectsUAClosure {
        e.subjectUAClosure(s, u)
    }
    
    // Warm operation caches
    for u, ops := range inv.subjectAllowOp {
        ua := e.subjectUAClosure(s, u)
        for op := range ops {
            e.allowedFor(s, u, op, ua)
        }
    }
}
```

## Next Steps

- See [CACHING.md](CACHING.md) for cache invalidation details
- See [ENGINE.md](ENGINE.md) for engine implementation
- See [DATABASE.md](DATABASE.md) for change log schema

