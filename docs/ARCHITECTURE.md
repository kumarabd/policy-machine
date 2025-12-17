# Architecture Overview

This document provides a comprehensive overview of the Policy Machine architecture, its components, data flow, and design decisions.

## Table of Contents

- [System Overview](#system-overview)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [NGAC Model Implementation](#ngac-model-implementation)
- [Design Principles](#design-principles)
- [Concurrency Model](#concurrency-model)
- [Performance Characteristics](#performance-characteristics)

## System Overview

Policy Machine is a production-grade authorization engine implementing the Next Generation Access Control (NGAC) model. The system is designed for:

- **High Throughput**: Sub-millisecond authorization decisions
- **Low Latency**: Microsecond-level response times for cached decisions
- **Consistency**: Strong consistency guarantees with incremental updates
- **Scalability**: Multi-tenant architecture with per-tenant isolation
- **Reliability**: Copy-on-write snapshots with graceful degradation

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Server Layer                        │
│  (Chi Router, Middleware, Request/Response Handling)        │
└──────────────────────┬──────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    Engine Layer                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   Snapshot   │  │   Closures    │  │   Decisions   │   │
│  │  Management  │  │  Computation  │  │   Evaluation  │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                           │
│  ┌────────────────────────────────────────────────────┐  │
│  │         Multi-Tier Caching System                 │  │
│  │  • UA/OA Closure Cache  • User-Op Bitmap Cache    │  │
│  │  • Node Closure Cache   • Decision Cache          │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────┬──────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                 PostgreSQL Database                         │
│  • Policy Graph Storage  • Change Log (Outbox Pattern)      │
│  • Revision Tracking    • Multi-tenant Isolation           │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Engine (`pkg/engine/`)

The engine is the heart of the authorization system, responsible for:

- **Snapshot Management**: Immutable, versioned policy graph snapshots
- **Decision Evaluation**: NGAC-compliant authorization decisions
- **Closure Computation**: Efficient graph traversals using Roaring Bitmaps
- **Cache Management**: Multi-tier caching with intelligent invalidation
- **Incremental Updates**: Copy-on-write snapshot updates

**Key Files:**
- `engine.go`: Main engine orchestration and decision logic
- `snapshot.go`: Immutable policy graph representation
- `load.go`: Snapshot loading from database
- `refresh.go`: Incremental snapshot updates
- `closures.go`: User/Object attribute closure computation
- `node_closure.go`: Node-level graph traversals

### 2. Database Layer (`pkg/postgres/`)

The database layer provides:

- **Schema Management**: GORM-based schema with auto-migration
- **Change Tracking**: Append-only change log (outbox pattern)
- **Query Optimization**: Strategic indexes for fast lookups
- **Multi-tenancy**: Tenant isolation at the database level

**Key Files:**
- `postgres.go`: Connection management and migrations
- `types.go`: GORM models for all NGAC entities
- `changes.go`: Change log schema and types
- `helper.go`: Policy operation helpers
- `outbox_helpers.go`: Change log utilities

### 3. Server Layer (`pkg/server/`)

The server layer handles:

- **HTTP API**: RESTful endpoints for authorization and policy management
- **Route Registration**: Chi router with middleware
- **Health Checks**: `/healthz` and `/readyz` endpoints
- **Metrics**: Prometheus-compatible metrics endpoint
- **Swagger**: API documentation generation

**Key Files:**
- `server.go`: Server initialization and lifecycle
- `base.go`: Route registration and middleware
- `base_handlers.go`: Health and metrics handlers
- `models.go`: API request/response models

### 4. Configuration (`internal/config/`)

Centralized configuration management:

- **YAML-based Config**: Human-readable configuration files
- **Environment Overrides**: Support for environment variable overrides
- **Validation**: Configuration validation on startup

### 5. Metrics (`internal/metrics/`)

Observability and monitoring:

- **Prometheus Integration**: Standard Prometheus metrics
- **Request Tracking**: HTTP request counters
- **Extensible**: Easy to add custom metrics

## Data Flow

### Authorization Decision Flow

```
1. HTTP Request
   └─> POST /api/v1/authorize
       { user_id, object_id, operation }

2. Server Handler
   └─> engine.Decide(ctx, userID, objectID, op)

3. Decision Cache Lookup
   └─> Check indexedDecisionCache
       ├─> HIT: Return cached decision (1-10μs)
       └─> MISS: Continue to computation

4. Snapshot Retrieval
   └─> engine.Snapshot() (atomic pointer read)
       ├─> NULL: Trigger Refresh() (full load)
       └─> Valid: Use current snapshot

5. Closure Computation
   ├─> userUAClosure(userID)
   │   └─> Check uaCache
   │       ├─> HIT: Return cached closure
   │       └─> MISS: Compute via graph traversal
   │           └─> Cache result (TTL: 2min)
   │
   └─> objectOAClosure(objectID)
       └─> Check oaCache
           ├─> HIT: Return cached closure
           └─> MISS: Compute via graph traversal
               └─> Cache result (TTL: 2min)

6. Policy Class Check
   └─> Compute userPCs ∩ objPCs
       └─> Empty? Return DENY (no shared policy class)

7. Allow/Deny Computation
   ├─> allowedFor(user, op, uaClosure)
   │   └─> Check allowCache
   │       ├─> HIT: Return cached bitmap
   │       └─> MISS: Aggregate associations
   │           └─> Cache result (TTL: 2min)
   │
   └─> deniedFor(user, op, uaClosure)
       └─> Check denyCache
           ├─> HIT: Return cached bitmap
           └─> MISS: Aggregate prohibitions
               └─> Cache result (TTL: 2min)

8. Final Decision
   └─> effective = (allowed - denied) ∩ oaClosure
       └─> !effective.IsEmpty() ? ALLOW : DENY

9. Cache Decision
   └─> decisions.Put(userID, objectID, op, allowed, revision)
       └─> TTL: 60 seconds

10. Response
    └─> { allowed: bool, revision: int64 }
```

### Policy Update Flow

```
1. Policy Change (via API or direct DB write)
   └─> Insert into policy_changes table
       └─> Increment policy_revisions.revision

2. Incremental Refresh Trigger
   └─> engine.RefreshIncremental(ctx)
       └─> Called periodically or on-demand

3. Change Detection
   └─> Query policy_changes WHERE seq > lastSeq
       └─> Ordered by seq ASC

4. Invalidation Computation
   └─> For each change:
       └─> computeInvalidations(inv, snapshot, change)
           └─> Track affected users, objects, operations

5. Copy-on-Write Snapshot Update
   └─> cloneSnapshotShallow(snapshot)
       └─> For each change:
           └─> applyChange(newSnapshot, change)
               └─> Clone only mutated maps/bitmaps

6. Atomic Snapshot Swap
   └─> engine.cur.Store(newSnapshot)
       └─> All new requests use new snapshot

7. Cache Invalidation
   └─> applyInvalidations(inv)
       └─> Delete affected cache entries
           ├─> UA/OA closures
           ├─> Allow/deny bitmaps
           └─> Decisions

8. Warmup (Optional)
   └─> Pre-compute closures for affected users/objects
       └─> Runs asynchronously
```

## NGAC Model Implementation

### Entity Types

1. **Users (U)**: Subject entities requesting access
   - Stored in `users` table
   - Identified by UUID
   - Multi-tenant isolation

2. **Objects (O)**: Protected resources
   - Stored in `objects` table
   - Identified by UUID
   - Can have type metadata

3. **User Attributes (UA)**: Hierarchical user groupings
   - Roles, teams, departments
   - Form a DAG (Directed Acyclic Graph)
   - Stored in `user_attributes` table

4. **Object Attributes (OA)**: Hierarchical resource groupings
   - Folders, projects, categories
   - Form a DAG
   - Stored in `object_attributes` table

5. **Policy Classes (PC)**: Isolated policy domains
   - Independent rule sets
   - Users and objects must share at least one PC
   - Stored in `policy_classes` table

### Graph Structure

```
Users ──┐
        ├──> UA ──> UA ──> UA ──┐
        │                        ├──> PC
Objects ──┐                      │
          ├──> OA ──> OA ──> OA ──┘
          │
          └──> Associations: UA ──[op]──> OA
```

### Assignment Edges

The system supports typed assignment edges:

- `USER → UA`: User assigned to user attribute
- `UA → UA`: Hierarchical UA relationships
- `OBJECT → OA`: Object assigned to object attribute
- `OA → OA`: Hierarchical OA relationships
- `UA → PC`: UA participates in policy class
- `OA → PC`: OA participates in policy class

### Associations

Associations grant permissions:
- Format: `UA → [operation] → OA`
- Multiple operations per association
- Stored in `associations` and `association_operations` tables

### Prohibitions

Prohibitions explicitly deny access:
- Can target users or UAs
- Format: `Subject → [operation] → OA`
- Stored in `prohibitions` and `prohibition_operations` tables

## Design Principles

### 1. Immutability

- **Snapshots are immutable**: Once created, never modified
- **Copy-on-write updates**: Only clone what changes
- **Atomic swaps**: New snapshots replace old ones atomically
- **Benefits**: Thread-safe reads, no locking for decisions

### 2. Dense Indexing

- **UUID → uint32 mapping**: Convert sparse UUIDs to dense indices
- **Bitmap operations**: Use Roaring Bitmaps for efficient set operations
- **Memory efficiency**: ~100-500 bytes per cached decision
- **Performance**: Fast bitmap intersections and unions

### 3. Multi-Tier Caching

- **Closure caches**: Cache transitive closures (TTL: 2-10 min)
- **Operation caches**: Cache allow/deny bitmaps (TTL: 2 min)
- **Decision cache**: Cache final decisions (TTL: 60 sec)
- **Granular invalidation**: Only invalidate affected entries

### 4. Incremental Updates

- **Change log**: Append-only log of all policy changes
- **Sequence numbers**: Monotonic sequence for ordering
- **Gap detection**: Fallback to full refresh on gaps
- **Copy-on-write**: Efficient updates without full rebuilds

### 5. Graceful Degradation

- **Full refresh fallback**: On errors or gaps, rebuild from scratch
- **Revision validation**: Cache entries validated against snapshot version
- **Timeout handling**: Database queries with timeouts
- **Error recovery**: Log errors and continue serving requests

## Concurrency Model

### Read Path (Authorization Decisions)

- **Lock-free reads**: Snapshot accessed via atomic pointer
- **No mutexes**: All caches use RWMutex for concurrent reads
- **Thread-safe**: Multiple goroutines can evaluate decisions simultaneously

### Write Path (Policy Updates)

- **Single writer**: `refreshMu` ensures only one refresh at a time
- **Atomic swap**: New snapshot replaces old one atomically
- **Cache invalidation**: Performed under write locks
- **Warmup**: Runs asynchronously after snapshot swap

### Cache Access

- **Read locks**: For cache lookups (concurrent)
- **Write locks**: For cache updates/invalidations (exclusive)
- **TTL-based expiration**: Lazy cleanup on access

## Performance Characteristics

### Latency Breakdown

| Path | Latency | Description |
|------|---------|-------------|
| **Hot Path** | 1-10μs | Fully cached decision |
| **Warm Path** | 50-200μs | Cached closures, fresh decision |
| **Cold Path** | 1-5ms | Includes database snapshot load |
| **Full Refresh** | 10-100ms | Depends on policy graph size |

### Memory Footprint

| Component | Size (10K entities) | Notes |
|-----------|-------------------|-------|
| Snapshot structure | 2-5 MB | Dense indices, maps |
| Roaring bitmaps | 50-200 KB | Sparse graphs |
| Cache overhead | 1-10 MB | Depends on hit patterns |
| **Total** | **3-15 MB** | Per tenant |

### Throughput

- **Decisions/sec**: 10K-100K+ (depends on cache hit rate)
- **Concurrent requests**: Limited by Go runtime (typically 10K+)
- **Database connections**: Pool of 100 max connections

### Scalability Considerations

1. **Horizontal Scaling**: Stateless engine instances
2. **Database Scaling**: Read replicas for snapshot loading
3. **Cache Scaling**: Consider Redis for distributed caching
4. **Partitioning**: Per-tenant policy graphs enable sharding

## Next Steps

- See [ENGINE.md](ENGINE.md) for detailed engine implementation
- See [CACHING.md](CACHING.md) for caching strategy details
- See [DATABASE.md](DATABASE.md) for database schema and queries
- See [INCREMENTAL_REFRESH.md](INCREMENTAL_REFRESH.md) for update mechanisms

