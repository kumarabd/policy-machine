# API Contract

## Design Principles

1. **API vs persistence separation**: `pkg/api` contains only HTTP DTOs. `internal/postgres` contains persistence models. `internal/api/mapper` converts between them.

2. **Simple resource model**: Subject and Object are **id, name, kind, metadata**. Attributes are **id, name, metadata**. No display names, external IDs, or tenant in the API contract.

3. **No tenant in API**: Tenant is not exposed in request/response bodies. The server may still use tenant from context for scoping internally.

4. **Client-agnostic JSON**: camelCase for JSON fields.

## Subject

- **Fields**: `id`, `name`, `kind`, `metadata` (map of string keys/values)
- **Create**: `name` (required), `kind`, `metadata`
- **Update**: `name`, `kind`, `metadata`

## Object

- **Fields**: `id`, `name`, `kind`, `metadata`
- **Create**: `name` (required), `kind`, `metadata`
- **Update**: `name`, `kind`, `metadata`

## Subject Attribute / Object Attribute

- **Fields**: `id`, `name`, `metadata`
- **Create**: `name` (required), `metadata`, optional `parentName`, `parentId` for hierarchy
- **Update**: `name`, `metadata`

(Attribute metadata is not yet persisted in the DB; API accepts it for future use.)

## Pagination

- `SearchResponse[T]`: `items`, `nextCursor`, `total`
- List responses: `cursor`, `hasMore`
