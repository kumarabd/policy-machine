# Database Schema and Query Patterns

This document covers the PostgreSQL database schema, indexing strategy, and query patterns used in Policy Machine.

## Table of Contents

- [Schema Overview](#schema-overview)
- [Core Tables](#core-tables)
- [Indexing Strategy](#indexing-strategy)
- [Query Patterns](#query-patterns)
- [Change Log Design](#change-log-design)
- [Multi-Tenancy](#multi-tenancy)
- [Performance Optimization](#performance-optimization)

## Schema Overview

Policy Machine uses PostgreSQL with GORM for ORM functionality. The schema implements the NGAC model with support for:

- Multi-tenant isolation
- Hierarchical user and object attributes
- Policy classes for domain isolation
- Associations for permission grants
- Prohibitions for explicit denials
- Change tracking for incremental updates

### Database Connection

Configured in `pkg/postgres/postgres.go`:

```go
type Handler struct {
    H  *gorm.DB
    db *sql.DB
}
```

**Connection Pool Settings**:
- Max idle connections: 10
- Max open connections: 100
- Connection lifetime: 10 seconds (configurable)
- Idle timeout: 10 seconds (configurable)

## Core Tables

### 1. Tenants

Multi-tenant isolation at the database level.

```sql
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR NOT NULL UNIQUE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

**Purpose**: Isolate policy data per tenant.

**Indexes**: `name` (unique)

### 2. Users

Subject entities requesting access.

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    external_id VARCHAR NOT NULL,
    email VARCHAR,
    display VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, external_id)` (unique, for external ID lookup)

**Purpose**: Store user entities with external identifiers.

### 3. Objects

Protected resources.

```sql
CREATE TABLE objects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    external_id VARCHAR NOT NULL,
    type VARCHAR NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, external_id)` (unique)
- `(tenant_id, type)` (for type-based queries)

**Purpose**: Store object entities with type metadata.

### 4. Policy Classes

Isolated policy domains.

```sql
CREATE TABLE policy_classes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, name)` (unique)

**Purpose**: Define policy domains that users and objects must share.

### 5. User Attributes (UA)

Hierarchical user groupings (roles, teams, departments).

```sql
CREATE TABLE user_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, name)` (unique)

**Purpose**: Represent roles, teams, or other user groupings.

### 6. Object Attributes (OA)

Hierarchical resource groupings (folders, projects, categories).

```sql
CREATE TABLE object_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, name)` (unique)

**Purpose**: Represent folders, projects, or other resource groupings.

### 7. Assignment Edges

Graph edges connecting users/objects to attributes and attributes to policy classes.

```sql
CREATE TABLE assignment_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    child_type VARCHAR NOT NULL,  -- USER, UA, OBJECT, OA, PC
    child_id UUID NOT NULL,
    parent_type VARCHAR NOT NULL,
    parent_id UUID NOT NULL,
    created_at TIMESTAMP,
    UNIQUE (tenant_id, child_type, child_id, parent_type, parent_id)
);
```

**Edge Types**:
- `USER → UA`: User assigned to user attribute
- `UA → UA`: Hierarchical UA relationships
- `OBJECT → OA`: Object assigned to object attribute
- `OA → OA`: Hierarchical OA relationships
- `UA → PC`: UA participates in policy class
- `OA → PC`: OA participates in policy class

**Indexes**:
- `(tenant_id, child_type, child_id)` (for child lookups)
- `(tenant_id, parent_type, parent_id)` (for parent lookups)
- `(tenant_id, child_type, child_id, parent_type, parent_id)` (unique)

**Purpose**: Represent the NGAC assignment graph.

### 8. Associations

Permission grants from UA to OA.

```sql
CREATE TABLE associations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_attribute_id UUID NOT NULL,
    object_attribute_id UUID NOT NULL,
    created_at TIMESTAMP,
    UNIQUE (tenant_id, user_attribute_id, object_attribute_id)
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, user_attribute_id)` (for UA lookups)
- `(tenant_id, object_attribute_id)` (for OA lookups)
- `(tenant_id, user_attribute_id, object_attribute_id)` (unique)

**Purpose**: Define which UAs can access which OAs.

### 9. Association Operations

Operations granted by associations.

```sql
CREATE TABLE association_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    association_id UUID NOT NULL,
    operation VARCHAR NOT NULL,
    created_at TIMESTAMP,
    UNIQUE (tenant_id, association_id, operation)
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `association_id` (for association lookups)
- `operation` (for operation-based queries)
- `(tenant_id, association_id, operation)` (unique)

**Purpose**: Specify which operations are granted by each association.

### 10. Prohibitions

Explicit access denials.

```sql
CREATE TABLE prohibitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    subject_type VARCHAR NOT NULL,  -- USER or UA
    subject_id UUID NOT NULL,
    object_attribute_id UUID NOT NULL,
    created_at TIMESTAMP
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `(tenant_id, subject_type, subject_id)` (for subject lookups)
- `(tenant_id, object_attribute_id)` (for OA lookups)

**Purpose**: Explicitly deny access for specific subjects and operations.

### 11. Prohibition Operations

Operations denied by prohibitions.

```sql
CREATE TABLE prohibition_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    prohibition_id UUID NOT NULL,
    operation VARCHAR NOT NULL,
    created_at TIMESTAMP,
    UNIQUE (tenant_id, prohibition_id, operation)
);
```

**Indexes**:
- `tenant_id` (for tenant filtering)
- `prohibition_id` (for prohibition lookups)
- `operation` (for operation-based queries)
- `(tenant_id, prohibition_id, operation)` (unique)

**Purpose**: Specify which operations are denied by each prohibition.

### 12. Policy Revisions

Global version counter per tenant.

```sql
CREATE TABLE policy_revisions (
    tenant_id UUID PRIMARY KEY,
    revision BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP
);
```

**Purpose**: Track policy version for cache invalidation and consistency.

**Updates**: Incremented on every policy change.

### 13. Policy Changes

Append-only change log (outbox pattern).

```sql
CREATE TABLE policy_changes (
    seq BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    revision BIGINT NOT NULL,
    kind VARCHAR NOT NULL,  -- ASSIGNMENT_EDGE, ASSOC_OP, PROHIB_OP
    op VARCHAR NOT NULL,  -- ADD, REMOVE, UPDATE
    payload JSONB NOT NULL,
    created_at TIMESTAMP
);
```

**Indexes**:
- `(tenant_id, seq)` (for ordered change retrieval)
- `(tenant_id, revision)` (for revision-based queries)

**Purpose**: Enable incremental snapshot updates.

**Change Kinds**:
- `ASSIGNMENT_EDGE`: Assignment edge added/removed
- `ASSOC_OP`: Association operation added/removed
- `PROHIB_OP`: Prohibition operation added/removed

## Indexing Strategy

### Composite Indexes

Most queries filter by `tenant_id` first, then by other fields. Composite indexes are optimized for this pattern:

```sql
-- Example: Find all assignment edges for a tenant and child
CREATE INDEX idx_asg_child_lookup 
ON assignment_edges (tenant_id, child_type, child_id);
```

### Unique Constraints

Enforced via unique indexes to prevent duplicate entries:

```sql
-- Example: Prevent duplicate assignment edges
CREATE UNIQUE INDEX uidx_asg_edge 
ON assignment_edges (tenant_id, child_type, child_id, parent_type, parent_id);
```

### Index Creation

Indexes are created automatically during database initialization (`pkg/postgres/postgres.go:100`):

```go
stmts := []string{
    `CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_tenant_external 
     ON users (tenant_id, external_id);`,
    // ... more indexes
}

for _, s := range stmts {
    if err := db.Exec(s).Error; err != nil {
        return nil, err
    }
}
```

## Query Patterns

### 1. Snapshot Loading

Load all policy data for a tenant:

```go
// Load UAs
var uas []postgres.UserAttribute
db.Where("tenant_id = ?", tenantID).Find(&uas)

// Load assignment edges
var edges []postgres.AssignmentEdge
db.Where("tenant_id = ?", tenantID).Find(&edges)

// Load associations
var assocs []postgres.Association
db.Where("tenant_id = ?", tenantID).Find(&assocs)
```

**Performance**: O(n) where n is the number of entities. Typically 10-100ms for 10K entities.

### 2. Change Log Queries

Retrieve changes since last sequence:

```go
var changes []postgres.PolicyChange
db.Where("tenant_id = ? AND seq > ?", tenantID, lastSeq)
  .Order("seq ASC")
  .Find(&changes)
```

**Performance**: O(k) where k is the number of changes. Typically <10ms for 100 changes.

**Index Used**: `(tenant_id, seq)`

### 3. Revision Check

Check current policy revision:

```go
var rev postgres.PolicyRevision
db.First(&rev, "tenant_id = ?", tenantID)
```

**Performance**: O(1) with primary key lookup. <1ms.

### 4. Child Lookups

Find all children of a node:

```go
var edges []postgres.AssignmentEdge
db.Where("tenant_id = ? AND parent_type = ? AND parent_id = ?", 
         tenantID, nodeType, nodeID)
  .Find(&edges)
```

**Index Used**: `(tenant_id, parent_type, parent_id)`

### 5. Parent Lookups

Find all parents of a node:

```go
var edges []postgres.AssignmentEdge
db.Where("tenant_id = ? AND child_type = ? AND child_id = ?", 
         tenantID, nodeType, nodeID)
  .Find(&edges)
```

**Index Used**: `(tenant_id, child_type, child_id)`

## Change Log Design

### Outbox Pattern

The `policy_changes` table implements the outbox pattern:

1. **Append-only**: Changes are never updated or deleted
2. **Ordered**: Sequence numbers ensure ordering
3. **Idempotent**: Changes can be replayed safely
4. **Immutable**: Once written, never modified

### Change Payload Structure

Changes store JSON payloads with all information needed to apply them:

```json
// ASSIGNMENT_EDGE
{
  "child_type": "USER",
  "child_id": "550e8400-...",
  "parent_type": "UA",
  "parent_id": "660e8400-..."
}

// ASSOC_OP
{
  "ua_id": "550e8400-...",
  "oa_id": "660e8400-...",
  "op": "read"
}

// PROHIB_OP
{
  "subject_type": "USER",
  "subject_id": "550e8400-...",
  "oa_id": "660e8400-...",
  "op": "write"
}
```

### Sequence Continuity

Sequence numbers must be continuous for incremental updates:

```go
func validateSeqContinuity(lastSeq int64, changes []postgres.PolicyChange) bool {
    if len(changes) == 0 {
        return true
    }
    if lastSeq != 0 && changes[0].Seq != lastSeq+1 {
        return false  // Gap detected
    }
    for i := 1; i < len(changes); i++ {
        if changes[i].Seq != changes[i-1].Seq+1 {
            return false  // Gap detected
        }
    }
    return true
}
```

**Gap Handling**: On gap detection, fallback to full refresh.

## Multi-Tenancy

### Tenant Isolation

All tables include `tenant_id` for isolation:

```go
type User struct {
    ID       uuid.UUID
    TenantID uuid.UUID `gorm:"index:idx_users_tenant"`
    // ...
}
```

### Query Filtering

All queries filter by `tenant_id`:

```go
db.Where("tenant_id = ?", tenantID).Find(&users)
```

### Foreign Key Constraints

Cascade deletes ensure tenant data is cleaned up:

```go
Tenant Tenant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
```

## Performance Optimization

### Connection Pooling

Configured for high concurrency:

```go
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Duration(opts.DialTimeoutSeconds) * time.Second)
```

### Query Optimization

1. **Use indexes**: All queries use indexed columns
2. **Batch loading**: Load related data in batches
3. **Select specific fields**: Avoid `SELECT *` when possible
4. **Limit results**: Use pagination for large result sets

### Index Maintenance

Indexes are automatically maintained by PostgreSQL. Monitor:

- **Index usage**: `pg_stat_user_indexes`
- **Index bloat**: `pg_stat_user_tables`
- **Query plans**: `EXPLAIN ANALYZE`

### Database Tuning

Recommended PostgreSQL settings:

```sql
-- Increase shared buffers for better caching
shared_buffers = 256MB

-- Increase work memory for sorts/joins
work_mem = 16MB

-- Enable query plan caching
plan_cache_mode = 'force_custom_plan'
```

## Migration Strategy

### Auto-Migration

GORM auto-migrates schema on startup:

```go
err = db.AutoMigrate(
    &Tenant{},
    &User{},
    &Object{},
    // ... all models
)
```

### Manual Migrations

For production, consider using a migration tool:

- **golang-migrate**: https://github.com/golang-migrate/migrate
- **atlas**: https://atlasgo.io/
- **goose**: https://github.com/pressly/goose

### Schema Evolution

When adding new fields:

1. Add field to model with default value
2. Run migration
3. Update application code
4. Deploy

## Backup and Recovery

### Backup Strategy

1. **Full backups**: Daily PostgreSQL dumps
2. **WAL archiving**: Continuous write-ahead log archiving
3. **Point-in-time recovery**: Restore to any point in time

### Recovery Procedures

1. Restore from backup
2. Replay WAL logs to desired point
3. Verify data integrity
4. Resume operations

## Monitoring

### Key Metrics

1. **Query latency**: Track slow queries
2. **Connection pool**: Monitor pool usage
3. **Index usage**: Ensure indexes are used
4. **Table sizes**: Monitor growth

### Query Performance

Use PostgreSQL's built-in tools:

```sql
-- Find slow queries
SELECT * FROM pg_stat_statements 
ORDER BY total_time DESC LIMIT 10;

-- Check index usage
SELECT * FROM pg_stat_user_indexes 
WHERE idx_scan = 0;
```

## Next Steps

- See [INCREMENTAL_REFRESH.md](INCREMENTAL_REFRESH.md) for change log usage
- See [ARCHITECTURE.md](ARCHITECTURE.md) for system overview
- See [ENGINE.md](ENGINE.md) for engine implementation

