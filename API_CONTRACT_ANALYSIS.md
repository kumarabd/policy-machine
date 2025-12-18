# API Contract Analysis: UI ↔ Backend

## Executive Summary

This document analyzes the contract between the Policy Studio UI and the Policy Machine backend API to identify inconsistencies and missing implementations.

## Critical Issues Found

### 1. ❌ SearchResponse Field Naming Inconsistency

**UI Expects:**
```typescript
interface SearchResponse<T> {
  items: T[]
  nextCursor?: string  // camelCase
  total?: number
}
```

**Backend Returns:**
```go
type SearchResponse[T any] struct {
    Items     []T    `json:"items"`
    NextCursor *string `json:"next_cursor,omitempty"`  // snake_case ❌
    Total     *int   `json:"total,omitempty"`
}
```

**Impact:** UI will not be able to read pagination cursor from responses.

**Fix Required:** Change `json:"next_cursor"` to `json:"nextCursor"` in `pkg/api/models.go`

---

### 2. ❌ EvaluateResponse Format Mismatch

**UI Expects:**
```typescript
interface EvaluateResponse {
  decision: "ALLOW" | "DENY"  // ❌ Backend returns "allowed" boolean
  versionId: string           // ❌ Backend returns "revision" number
  evaluatedAt: string          // ❌ Missing
  trace?: ExplainTrace         // ❌ Different structure
}
```

**Backend Returns:**
```go
type EvaluateResponse struct {
    Allowed     bool                  `json:"allowed"`     // ❌ Should be "decision"
    Revision    int64                 `json:"revision"`    // ❌ Should be "versionId" (string)
    Explanation *AuthorizationExplain `json:"explanation,omitempty"` // ❌ Different structure
}
```

**Impact:** UI cannot properly display evaluation results. The response format is completely incompatible.

**Fix Required:** 
- Add `Decision` field with "ALLOW"/"DENY" values
- Change `Revision` to `VersionID` (string)
- Add `EvaluatedAt` timestamp
- Restructure `Explanation` to match `ExplainTrace` format

---

### 3. ❌ Version Response Format Inconsistency

**UI Expects:**
```typescript
interface SearchResponse<Version> {
  items: Version[]
  nextCursor?: string  // camelCase
  total?: number
}
```

**Backend Returns:**
```go
type ListVersionsResponse struct {
    Items     []Version `json:"items"`
    NextCursor string   `json:"next_cursor,omitempty"`  // ❌ snake_case, not pointer
    Total     int       `json:"total"`                  // ❌ Not pointer, always present
}
```

**Impact:** Pagination and total count handling will fail.

**Fix Required:** 
- Use `SearchResponse[Version]` instead of custom type
- Fix field naming to camelCase
- Make `Total` optional pointer

---

### 4. ❌ Missing Endpoints

The UI expects these endpoints that are **NOT implemented** in the backend:

#### 4.1 Subject Set Member Management
- `POST /v1/subject-sets/{id}/members:add` - Add members to subject set
- `POST /v1/subject-sets/{id}/members:remove` - Remove members from subject set

#### 4.2 Object Set Member Management
- `POST /v1/object-sets/{id}/members:add` - Add members to object set
- `POST /v1/object-sets/{id}/members:remove` - Remove members from object set

#### 4.3 Version Management
- `GET /v1/versions/{id}/diff` - Get version diff
- `GET /v1/versions/{id}/snapshot` - Get policy snapshot at version

**Impact:** UI features for managing set members and viewing version details will not work.

---

### 5. ⚠️ Field Naming Inconsistencies

#### 5.1 Mixed Naming Conventions

**Backend uses snake_case in:**
- `next_cursor` (should be `nextCursor`)
- `created_at` (should be `createdAt`)
- `updated_at` (should be `updatedAt`)
- `scope_id` (should be `scopeId`)
- `subject_selector` (should be `subjectSelector`)
- `object_selector` (should be `objectSelector`)
- `change_count` (should be `changeCount`)
- `parent_version_id` (should be `parentVersionId`)

**Backend uses camelCase in:**
- `displayName` ✅
- `memberSubjectIds` ✅
- `memberObjectIds` ✅
- `scopeId` ✅ (in some places)

**Impact:** Inconsistent API surface makes it harder for UI to consume.

**Recommendation:** Standardize on camelCase for all JSON fields (JavaScript convention).

---

### 6. ⚠️ Subject/Object Field Mismatches

**UI Expects:**
```typescript
interface Subject {
  id: string
  displayName: string
  kind?: string
  attributes?: Record<string, string>
  tags?: string[]
  createdAt?: string
}
```

**Backend Returns:**
```go
type Subject struct {
    ID          uuid.UUID         `json:"id"`           // ✅ UUID (string in JSON)
    DisplayName string            `json:"displayName"`   // ✅
    Kind        string            `json:"kind,omitempty"` // ✅
    Attributes  map[string]string `json:"attributes,omitempty"` // ✅
    Tags        []string          `json:"tags,omitempty"` // ✅
    CreatedAt   time.Time         `json:"createdAt,omitempty"` // ✅ (ISO string in JSON)
    // Additional fields not in UI:
    ExternalID  string            `json:"external_id,omitempty"` // ⚠️ snake_case
    Email       string            `json:"email,omitempty"`
    Display     string            `json:"display,omitempty"`
}
```

**Status:** Mostly compatible, but extra fields may cause confusion.

---

### 7. ⚠️ Rule Field Naming

**UI Expects:**
```typescript
interface Rule {
  id: string
  name: string
  description?: string
  scopeId?: string  // camelCase
  actions: string[]
  subjectSelector: SubjectSelector
  objectSelector: ObjectSelector
  condition?: RuleCondition
  effect: RuleEffect
  priority?: number
  enabled: boolean
  createdAt?: string
  updatedAt?: string
}
```

**Backend Returns:**
```go
type Rule struct {
    ID              uuid.UUID      `json:"id"`
    Name            string         `json:"name"`
    Description     string         `json:"description,omitempty"`
    ScopeID         *uuid.UUID     `json:"scope_id,omitempty"`  // ❌ snake_case
    Actions         []string       `json:"actions"`
    SubjectSelector NodeRef        `json:"subjectSelector"`      // ✅
    ObjectSelector  NodeRef        `json:"objectSelector"`       // ✅
    Condition       *RuleCondition `json:"condition,omitempty"`
    Effect          string         `json:"effect"`
    Priority        *int           `json:"priority,omitempty"`
    Enabled         bool           `json:"enabled"`
    CreatedAt       time.Time      `json:"created_at,omitempty"`  // ❌ snake_case
    UpdatedAt       *time.Time     `json:"updated_at,omitempty"`   // ❌ snake_case
}
```

**Impact:** UI will not be able to read `scopeId`, `createdAt`, `updatedAt` fields correctly.

---

### 8. ⚠️ Version Field Naming

**UI Expects:**
```typescript
interface Version {
  id: string
  createdAt: string
  author?: string
  message?: string
  changeCount: number
  parentVersionId?: string  // camelCase
}
```

**Backend Returns:**
```go
type Version struct {
    ID              string    `json:"id"`
    CreatedAt       time.Time `json:"created_at"`        // ❌ snake_case
    Author          string    `json:"author,omitempty"`
    Message         string    `json:"message,omitempty"`
    ChangeCount     int       `json:"change_count"`       // ❌ snake_case
    ParentVersionID string    `json:"parent_version_id,omitempty"` // ❌ snake_case
}
```

**Impact:** UI cannot read `createdAt`, `changeCount`, `parentVersionId` fields.

---

## Summary of Required Fixes

### High Priority (Breaking)
1. ✅ Fix `SearchResponse` field: `next_cursor` → `nextCursor`
2. ✅ Fix `EvaluateResponse` format to match UI expectations
3. ✅ Fix `Version` field naming (snake_case → camelCase)
4. ✅ Fix `Rule` field naming (`scope_id`, `created_at`, `updated_at`)

### Medium Priority (Missing Features)
5. ✅ Implement `/v1/subject-sets/{id}/members:add`
6. ✅ Implement `/v1/subject-sets/{id}/members:remove`
7. ✅ Implement `/v1/object-sets/{id}/members:add`
8. ✅ Implement `/v1/object-sets/{id}/members:remove`
9. ✅ Implement `/v1/versions/{id}/diff`
10. ✅ Implement `/v1/versions/{id}/snapshot`

### Low Priority (Consistency)
11. ⚠️ Standardize all field names to camelCase
12. ⚠️ Review and align all response types with UI expectations

---

## Testing Recommendations

After fixes are applied:
1. Test all API endpoints with UI client code
2. Verify pagination works correctly
3. Test evaluate endpoint with explain=true
4. Test version diff and snapshot endpoints
5. Test set member management operations

---

## Notes

- The backend uses Go's `uuid.UUID` type which serializes to string in JSON, so that's compatible.
- `time.Time` serializes to RFC3339 string in JSON, which is compatible with TypeScript `string` type.
- The UI uses `/v1/` prefix, backend supports both `/v1/` and `/api/v1/` - this is fine.

