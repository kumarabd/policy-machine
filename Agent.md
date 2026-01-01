# Cursor AI Agent Rules for Policy Machine

This document defines coding standards, architectural patterns, and implementation rules for the Policy Machine codebase. Use this as a reference when making changes or adding features.

## Table of Contents

- [Code Style](#code-style)
- [Architectural Principles](#architectural-principles)
- [Package Organization](#package-organization)
- [Error Handling](#error-handling)
- [Concurrency Patterns](#concurrency-patterns)
- [Testing Standards](#testing-standards)
- [Documentation Requirements](#documentation-requirements)

## Code Style

### Go Conventions

1. **Follow standard Go formatting**: Use `gofmt` or `goimports`
2. **Naming conventions**:
   - Exported types/functions: PascalCase
   - Unexported: camelCase
   - Constants: PascalCase or UPPER_SNAKE_CASE
   - Interfaces: End with `-er` when appropriate (e.g., `ObligationEmitter`)

3. **File organization**:
   - Package declaration
   - Imports (stdlib, third-party, local)
   - Constants
   - Types
   - Functions

4. **Line length**: Prefer <100 characters, but readability over strict limits

### Code Examples

```go
// ✅ Good: Clear, descriptive names
func (e *Engine) subjectUAClosure(s *Snapshot, subjectID uuid.UUID) *roaring.Bitmap {
    // ...
}

// ❌ Bad: Abbreviated, unclear
func (e *Engine) uaClos(u uuid.UUID) *roaring.Bitmap {
    // ...
}
```

## Architectural Principles

### 1. Immutability

**Rule**: Snapshots and cached data structures should be immutable after creation.

**Implementation**:
- Use copy-on-write for updates
- Clone bitmaps before mutation
- Clone maps before modification
- Never modify shared data structures

**Example**:
```go
// ✅ Good: Clone before mutation
b := s.assoc[uaIdx][op]
if b == nil {
    b = roaring.New()
} else {
    b = b.Clone()  // Clone immutable bitmap
}
b.Add(oaIdx)

// ❌ Bad: Direct mutation
s.assoc[uaIdx][op].Add(oaIdx)  // Mutates shared bitmap
```

### 2. Atomic Operations

**Rule**: Use atomic operations for shared state that's read frequently.

**Implementation**:
- Use `atomic.Pointer` for snapshot references
- Use `sync.RWMutex` for cache access
- Prefer read locks over write locks

**Example**:
```go
// ✅ Good: Atomic snapshot pointer
type Engine struct {
    cur atomic.Pointer[Snapshot]
}

func (e *Engine) Snapshot() *Snapshot {
    return e.cur.Load()  // Lock-free read
}

// ❌ Bad: Mutex for snapshot
type Engine struct {
    mu sync.Mutex
    cur *Snapshot
}
```

### 3. Copy-on-Write

**Rule**: Only clone data structures when you need to modify them.

**Implementation**:
- Shallow clone snapshots
- Clone maps only when mutating
- Clone bitmaps only when modifying
- Preserve immutability of unchanged data

**Example**:
```go
// ✅ Good: Clone only what changes
func applyChange(s *Snapshot, ch PolicyChange) {
    next := cloneSnapshotShallow(s)  // Shallow copy
    if needsMutation {
        next.someMap = cowMap(s.someMap)  // Clone only this map
    }
}

// ❌ Bad: Deep clone everything
func applyChange(s *Snapshot, ch PolicyChange) {
    next := deepClone(s)  // Expensive, unnecessary
}
```

### 4. Granular Invalidation

**Rule**: Only invalidate cache entries that are actually affected by changes.

**Implementation**:
- Compute affected entities during change application
- Invalidate specific cache entries, not entire caches
- Use indexes for fast invalidation (by subject, object, operation)

**Example**:
```go
// ✅ Good: Granular invalidation
if subjectAssignedToUA {
    inv.subjectsUAClosure[subjectID] = struct{}{}
    // Later: Only invalidate this subject's caches
    e.uaCache.Delete(subjectID)
    e.decisions.DeleteSubject(subjectID)
}

// ❌ Bad: Clear entire cache
if subjectAssignedToUA {
    e.uaCache.Clear()  // Invalidates all subjects
    e.decisions.Clear()  // Invalidates all decisions
}
```

### 5. Graceful Degradation

**Rule**: Always have a fallback path when incremental updates fail.

**Implementation**:
- Fallback to full refresh on errors
- Validate sequence continuity
- Check revision monotonicity
- Log errors but continue serving requests

**Example**:
```go
// ✅ Good: Fallback on error
func (e *Engine) RefreshIncremental(ctx context.Context) error {
    // ... incremental logic
    if err != nil {
        return e.Refresh(ctx)  // Fallback to full refresh
    }
    if !validateSeqContinuity(lastSeq, changes) {
        return e.Refresh(ctx)  // Gap detected -> full refresh
    }
}

// ❌ Bad: Fail on error
func (e *Engine) RefreshIncremental(ctx context.Context) error {
    if err != nil {
        return err  // No fallback
    }
}
```

## Package Organization

### Package Structure

```
pkg/
├── engine/          # Core authorization engine
│   ├── engine.go    # Main engine type and decision logic
│   ├── snapshot.go  # Immutable snapshot structure
│   ├── load.go      # Snapshot loading from database
│   ├── refresh.go    # Incremental refresh logic
│   ├── apply.go     # Change application
│   ├── invalidate.go # Cache invalidation computation
│   ├── closures.go  # Closure computation
│   ├── node_closure.go # Node-level traversals
│   ├── closure_cache.go # Closure cache implementation
│   ├── subjectop_cache.go # Operation cache implementation
│   ├── decision_cache.go # Decision cache implementation
│   └── emitter.go   # Obligation/event emission
├── postgres/        # Database layer
│   ├── postgres.go  # Connection and migrations
│   ├── types.go     # GORM models
│   ├── changes.go   # Change log types
│   ├── helper.go    # Policy operation helpers
│   └── outbox_helpers.go # Change log utilities
└── server/          # HTTP server
    ├── server.go    # Server initialization
    ├── base.go      # Route registration
    ├── base_handlers.go # Health/metrics handlers
    └── models.go    # API request/response models
```

### Package Rules

1. **Single Responsibility**: Each package has a clear, single purpose
2. **No Circular Dependencies**: Packages should not import each other circularly
3. **Internal Packages**: Use `internal/` for application-specific code
4. **Public API**: Only export what's needed by other packages

### Import Organization

```go
import (
    // Standard library
    "context"
    "sync"
    "time"
    
    // Third-party
    "github.com/RoaringBitmap/roaring"
    "github.com/google/uuid"
    
    // Local
    "github.com/kumarabd/policy-machine/pkg/postgres"
)
```

## Error Handling

### Error Patterns

1. **Return Errors**: Don't panic, return errors
2. **Error Context**: Wrap errors with context
3. **Error Types**: Use sentinel errors for common cases
4. **Logging**: Log errors at appropriate levels

**Example**:
```go
// ✅ Good: Return error with context
func (e *Engine) Refresh(ctx context.Context) error {
    snap, err := LoadSnapshot(ctx, e.db.H, e.tenantID)
    if err != nil {
        return fmt.Errorf("failed to load snapshot: %w", err)
    }
    // ...
}

// ❌ Bad: Panic on error
func (e *Engine) Refresh(ctx context.Context) {
    snap := LoadSnapshot(ctx, e.db.H, e.tenantID)  // No error handling
    // ...
}
```

### Error Recovery

- **Retry Logic**: Use exponential backoff for transient errors
- **Fallback**: Always have a fallback path (e.g., full refresh)
- **Graceful Degradation**: Continue serving requests even on errors

## Concurrency Patterns

### Locking Strategy

1. **Read Locks**: Use `RLock()` for cache reads
2. **Write Locks**: Use `Lock()` for cache writes
3. **Lock-Free Reads**: Use atomic operations when possible
4. **Lock Ordering**: Always acquire locks in the same order

**Example**:
```go
// ✅ Good: Read lock for cache access
func (c *closureCache[K]) Get(k K) (*roaring.Bitmap, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    // ... read operations
}

// ✅ Good: Atomic for snapshot
func (e *Engine) Snapshot() *Snapshot {
    return e.cur.Load()  // Lock-free
}
```

### Goroutine Safety

- **Snapshot Access**: Safe to read from multiple goroutines (immutable)
- **Cache Access**: Protected by RWMutex
- **Refresh**: Serialized by `refreshMu` mutex

## Testing Standards

### Test Organization

1. **Test Files**: `*_test.go` files in same package
2. **Test Functions**: `TestXxx` for unit tests
3. **Table-Driven Tests**: Use for multiple test cases
4. **Test Helpers**: Extract common test logic

**Example**:
```go
func TestSubjectUAClosure(t *testing.T) {
    tests := []struct {
        name     string
        subjectID   uuid.UUID
        expected []uint32
    }{
        {
            name:     "subject with single UA",
            subjectID:   subject1,
            expected: []uint32{0, 1},
        },
        // ... more cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### Test Coverage

- **Aim for >80% coverage** for critical paths
- **Test edge cases**: Empty sets, nil values, errors
- **Test concurrency**: Race conditions, concurrent access
- **Test performance**: Benchmark critical operations

## Documentation Requirements

### Code Comments

1. **Package Comments**: Document package purpose
2. **Exported Types/Functions**: Document all exported symbols
3. **Complex Logic**: Explain non-obvious algorithms
4. **Examples**: Provide usage examples for complex functions

**Example**:
```go
// subjectUAClosure returns the set of all Subject Attributes (UAs) reachable
// from the given subject via assignment edges. The result includes the subject's
// direct UA assignments and all ancestor UAs (transitive closure).
//
// The closure is cached with a TTL of 2 minutes. If the cache is empty
// or expired, the closure is computed via BFS traversal of the UA graph.
func (e *Engine) subjectUAClosure(s *Snapshot, subjectID uuid.UUID) *roaring.Bitmap {
    // ...
}
```

### Documentation Files

- **README.md**: Project overview and quick start
- **docs/ARCHITECTURE.md**: System design and components
- **docs/ENGINE.md**: Engine implementation details
- **docs/CACHING.md**: Caching strategy
- **docs/DATABASE.md**: Database schema
- **docs/INCREMENTAL_REFRESH.md**: Update mechanisms

## Performance Guidelines

### Optimization Principles

1. **Measure First**: Profile before optimizing
2. **Cache Wisely**: Cache expensive computations
3. **Avoid Allocations**: Reuse buffers, minimize allocations
4. **Batch Operations**: Group database queries when possible

### Performance Targets

- **Hot Path**: 1-10μs (cached decisions)
- **Warm Path**: 50-200μs (cached closures)
- **Cold Path**: 1-5ms (full computation)
- **Refresh**: 10-100ms (incremental), 100-1000ms (full)

## Database Patterns

### Query Patterns

1. **Always Filter by Tenant**: All queries must include `tenant_id`
2. **Use Indexes**: Query indexed columns
3. **Batch Loading**: Load related data in batches
4. **Connection Pooling**: Use connection pool, don't create new connections

**Example**:
```go
// ✅ Good: Filter by tenant, use index
db.Where("tenant_id = ?", tenantID).
   Where("seq > ?", lastSeq).
   Order("seq ASC").
   Find(&changes)

// ❌ Bad: No tenant filter, no index
db.Find(&changes)  // Scans entire table
```

### Change Logging

1. **Always Log Changes**: Every policy modification must create a change log entry
2. **Increment Revision**: Update `policy_revisions` on every change
3. **Atomic Operations**: Use transactions for multi-step changes
4. **Sequence Continuity**: Ensure sequence numbers are continuous

## Security Considerations

1. **Input Validation**: Validate all subject inputs
2. **SQL Injection**: Use parameterized queries (GORM handles this)
3. **Tenant Isolation**: Never leak data across tenants
4. **Authorization**: Validate permissions before policy changes

## Migration Guidelines

### Adding New Features

1. **Backward Compatibility**: Maintain compatibility with existing data
2. **Migration Scripts**: Create migration scripts for schema changes
3. **Feature Flags**: Use feature flags for gradual rollouts
4. **Documentation**: Update documentation for new features

### Breaking Changes

1. **Version Bumping**: Bump major version for breaking changes
2. **Deprecation Period**: Deprecate old APIs before removal
3. **Migration Guide**: Provide migration guide for subjects
4. **Communication**: Clearly communicate breaking changes

## Code Review Checklist

Before submitting code, ensure:

- [ ] Code follows Go conventions and passes `golangci-lint`
- [ ] All exported symbols are documented
- [ ] Tests are included for new functionality
- [ ] Error handling is comprehensive
- [ ] Concurrency is handled correctly
- [ ] Performance is acceptable (benchmarked if critical)
- [ ] Documentation is updated
- [ ] No breaking changes (or properly versioned)

## Common Pitfalls to Avoid

1. **Mutating Shared Data**: Always clone before mutation
2. **Forgetting Tenant Filter**: Always filter by `tenant_id`
3. **Cache Invalidation**: Don't forget to invalidate affected caches
4. **Error Handling**: Don't ignore errors, handle them appropriately
5. **Lock Ordering**: Avoid deadlocks by consistent lock ordering
6. **Memory Leaks**: Clean up resources, don't hold references unnecessarily

## References

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [NGAC Specification](https://nvlpubs.nist.gov/nistpubs/specialpublications/NIST.sp.800-162.pdf)
- Project documentation in `docs/` directory

