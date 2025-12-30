package api

import (
	"time"

	"github.com/google/uuid"
)

// --- Generic Search Response ---
type SearchResponse[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor,omitempty"`
	Total      *int    `json:"total,omitempty"`
}

// --- Error Response ---
type ErrorResponse struct {
	Error struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details,omitempty"`
	} `json:"error"`
}

// --- Metadata & Revision ---
type MetaResponse struct {
	ServiceVersion string    `json:"service_version"`
	TenantID       string    `json:"tenant_id"`
	Revision       int64     `json:"revision"`
	AppliedSeq     int64     `json:"applied_seq"`
	Now            time.Time `json:"now"`
}

type RevisionResponse struct {
	TenantID string `json:"tenant_id"`
	Revision int64  `json:"revision"`
}

type PolicyChangeItem struct {
	Seq       int64                  `json:"seq"`
	Revision  int64                  `json:"revision"`
	Kind      string                 `json:"kind"`
	Op        string                 `json:"op"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

type ChangesResponse struct {
	TenantID string             `json:"tenant_id"`
	FromSeq  int64              `json:"from_seq"`
	ToSeq    int64              `json:"to_seq"`
	Changes  []PolicyChangeItem `json:"changes"`
}

// Version represents a policy version (derived from revisions)
type Version struct {
	ID              string    `json:"id"` // Revision number as string
	CreatedAt       time.Time `json:"createdAt"`
	Author          string    `json:"author,omitempty"`
	Message         string    `json:"message,omitempty"`
	ChangeCount     int       `json:"changeCount"`
	ParentVersionID *string   `json:"parentVersionId,omitempty"`
}

// ListVersionsResponse uses the generic SearchResponse
type ListVersionsResponse = SearchResponse[Version]

// --- Subjects (Users) ---
type Subject struct {
	ID          uuid.UUID         `json:"id"`
	ExternalID  string            `json:"external_id,omitempty"`
	Email       string            `json:"email,omitempty"`
	Display     string            `json:"display,omitempty"`
	DisplayName string            `json:"displayName"` // UI expects this
	Kind        string            `json:"kind,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"createdAt,omitempty"`
}

type CreateSubjectRequest struct {
	ExternalID string `json:"external_id"`
	Email      string `json:"email,omitempty"`
	Display    string `json:"display,omitempty"`
}

type UpdateSubjectRequest struct {
	Email   string `json:"email,omitempty"`
	Display string `json:"display,omitempty"`
}

type SubjectResponse struct {
	Subject  Subject `json:"subject"`
	Revision int64   `json:"revision"`
}

type ListSubjectsResponse struct {
	Subjects []Subject `json:"subjects"`
	Cursor   string    `json:"cursor,omitempty"`
	HasMore  bool      `json:"has_more"`
}

// --- Subject Sets (UAs) ---
type SubjectSet struct {
	ID               uuid.UUID   `json:"id"`
	Name             string      `json:"name"`
	Description      string      `json:"description,omitempty"`
	ScopeID          *uuid.UUID  `json:"scopeId,omitempty"`
	Tags             []string    `json:"tags,omitempty"`
	MemberSubjectIDs []uuid.UUID `json:"memberSubjectIds,omitempty"`
	CreatedAt        time.Time   `json:"createdAt,omitempty"`
	UpdatedAt        *time.Time  `json:"updatedAt,omitempty"`
}

// SubjectGroup is an alias for SubjectSet (backward compatibility)
type SubjectGroup = SubjectSet

type CreateSubjectSetRequest struct {
	Name string `json:"name"`
}

// CreateSubjectGroupRequest is an alias for CreateSubjectSetRequest (backward compatibility)
type CreateSubjectGroupRequest = CreateSubjectSetRequest

type UpdateSubjectSetRequest struct {
	Name string `json:"name"`
}

// UpdateSubjectGroupRequest is an alias for UpdateSubjectSetRequest (backward compatibility)
type UpdateSubjectGroupRequest = UpdateSubjectSetRequest

type SubjectSetResponse struct {
	Group    SubjectSet `json:"group"`
	Revision int64      `json:"revision"`
}

// SubjectGroupResponse is an alias for SubjectSetResponse (backward compatibility)
type SubjectGroupResponse = SubjectSetResponse

type ListSubjectSetsResponse struct {
	Groups  []SubjectSet `json:"groups"`
	Cursor  string       `json:"cursor,omitempty"`
	HasMore bool         `json:"has_more"`
}

// ListSubjectGroupsResponse is an alias for ListSubjectSetsResponse (backward compatibility)
type ListSubjectGroupsResponse = ListSubjectSetsResponse

// --- Objects ---
type Object struct {
	ID          uuid.UUID         `json:"id"`
	ExternalID  string            `json:"external_id,omitempty"`
	Type        string            `json:"type,omitempty"`
	DisplayName string            `json:"displayName"` // UI expects this
	Kind        string            `json:"kind,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"createdAt,omitempty"`
}

type CreateObjectRequest struct {
	ExternalID string `json:"external_id"`
	Type       string `json:"type,omitempty"`
}

type UpdateObjectRequest struct {
	Type string `json:"type,omitempty"`
}

type ObjectResponse struct {
	Object   Object `json:"object"`
	Revision int64  `json:"revision"`
}

type ListObjectsResponse struct {
	Objects []Object `json:"objects"`
	Cursor  string   `json:"cursor,omitempty"`
	HasMore bool     `json:"has_more"`
}

// --- Object Sets (OAs) ---
type ObjectSet struct {
	ID              uuid.UUID   `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description,omitempty"`
	ScopeID         *uuid.UUID  `json:"scopeId,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
	MemberObjectIDs []uuid.UUID `json:"memberObjectIds,omitempty"`
	CreatedAt       time.Time   `json:"createdAt,omitempty"`
	UpdatedAt       *time.Time  `json:"updatedAt,omitempty"`
}

// ObjectGroup is an alias for ObjectSet (backward compatibility)
type ObjectGroup = ObjectSet

type CreateObjectSetRequest struct {
	Name string `json:"name"`
}

// CreateObjectGroupRequest is an alias for CreateObjectSetRequest (backward compatibility)
type CreateObjectGroupRequest = CreateObjectSetRequest

type UpdateObjectSetRequest struct {
	Name string `json:"name"`
}

// UpdateObjectGroupRequest is an alias for UpdateObjectSetRequest (backward compatibility)
type UpdateObjectGroupRequest = UpdateObjectSetRequest

type ObjectSetResponse struct {
	Group    ObjectSet `json:"group"`
	Revision int64     `json:"revision"`
}

// ObjectGroupResponse is an alias for ObjectSetResponse (backward compatibility)
type ObjectGroupResponse = ObjectSetResponse

type ListObjectSetsResponse struct {
	Groups  []ObjectSet `json:"groups"`
	Cursor  string      `json:"cursor,omitempty"`
	HasMore bool        `json:"has_more"`
}

// ListObjectGroupsResponse is an alias for ListObjectSetsResponse (backward compatibility)
type ListObjectGroupsResponse = ListObjectSetsResponse

// --- Relationships (Assignment Edges) ---
type NodeRef struct {
	Type string    `json:"type"` // "subject", "subject-set", "object", "object-set"
	ID   uuid.UUID `json:"id"`
}

type Relationship struct {
	ID   uuid.UUID `json:"id,omitempty"`
	Kind string    `json:"kind"` // "subject_member_of_set", "subject_set_parent_of_set", etc.
	From NodeRef   `json:"from"`
	To   NodeRef   `json:"to"`
}

type CreateRelationshipRequest struct {
	Kind string  `json:"kind"`
	From NodeRef `json:"from"`
	To   NodeRef `json:"to"`
}

type DeleteRelationshipRequest struct {
	Kind string  `json:"kind"`
	From NodeRef `json:"from"`
	To   NodeRef `json:"to"`
}

type RelationshipResponse struct {
	Relationship Relationship `json:"relationship"`
	Revision     int64        `json:"revision"`
}

type ListRelationshipsResponse struct {
	Relationships []Relationship `json:"relationships"`
	Cursor        string         `json:"cursor,omitempty"`
	HasMore       bool           `json:"has_more"`
}

// --- Rules (Associations) ---
// Scope is used in Deny rules (legacy)
type Scope struct {
	Type string    `json:"type"` // "subject-set", "object-set"
	ID   uuid.UUID `json:"id"`
}

// PolicyScope represents an organizational scope for grouping rules, subjects, and objects
type PolicyScope struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
}

type Rule struct {
	ID              uuid.UUID      `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	ScopeID         *uuid.UUID     `json:"scopeId,omitempty"`
	Actions         []string       `json:"actions"`
	SubjectSelector NodeRef        `json:"subjectSelector"`
	ObjectSelector  NodeRef        `json:"objectSelector"`
	Condition       *RuleCondition `json:"condition,omitempty"`
	Effect          string         `json:"effect"` // "ALLOW", "DENY", etc.
	Priority        *int           `json:"priority,omitempty"`
	Enabled         bool           `json:"enabled"`
	CreatedAt       time.Time      `json:"createdAt,omitempty"`
	UpdatedAt       *time.Time     `json:"updatedAt,omitempty"`
}

type RuleCondition struct {
	Type string      `json:"type"` // "JSON_LOGIC", "RAW"
	Expr interface{} `json:"expr"`
}

type CreateRuleRequest struct {
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	ScopeID         *uuid.UUID     `json:"scopeId,omitempty"`
	Actions         []string       `json:"actions"`
	SubjectSelector NodeRef        `json:"subjectSelector"`
	ObjectSelector  NodeRef        `json:"objectSelector"`
	Condition       *RuleCondition `json:"condition,omitempty"`
	Effect          string         `json:"effect"` // "ALLOW", "DENY", etc.
	Priority        *int           `json:"priority,omitempty"`
	Enabled         bool           `json:"enabled"`
}

type UpdateRuleRequest struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Actions     []string       `json:"actions,omitempty"`
	Condition   *RuleCondition `json:"condition,omitempty"`
	Effect      *string        `json:"effect,omitempty"`
	Priority    *int           `json:"priority,omitempty"`
	Enabled     *bool          `json:"enabled,omitempty"`
}

type RuleResponse struct {
	Rule     Rule  `json:"rule"`
	Revision int64 `json:"revision"`
}

// ListRulesResponse is deprecated, use SearchResponse[Rule] instead
type ListRulesResponse struct {
	Rules   []Rule `json:"rules"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

// --- Denies (Prohibitions) ---
type Deny struct {
	ID          uuid.UUID `json:"id"`
	Subject     NodeRef   `json:"subject"` // type: "subject" or "subject-set"
	Operations  []string  `json:"operations"`
	Targets     []Scope   `json:"targets"` // object-sets
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateDenyRequest struct {
	Subject     NodeRef  `json:"subject"`
	Operations  []string `json:"operations"`
	Targets     []Scope  `json:"targets"`
	Description string   `json:"description,omitempty"`
}

type UpdateDenyRequest struct {
	Operations  []string `json:"operations,omitempty"`
	Targets     []Scope  `json:"targets,omitempty"`
	Description string   `json:"description,omitempty"`
}

type DenyResponse struct {
	Deny     Deny  `json:"deny"`
	Revision int64 `json:"revision"`
}

type ListDeniesResponse struct {
	Denies  []Deny `json:"denies"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

// --- Authorization ---
type AuthorizeRequest struct {
	UserID    uuid.UUID `json:"user_id"`
	ObjectID  uuid.UUID `json:"object_id"`
	Operation string    `json:"operation"`
}

type AuthorizeResponse struct {
	Allowed  bool  `json:"allowed"`
	Revision int64 `json:"revision"`
}

// --- Evaluate (UI-compatible format) ---
type EvaluateRequest struct {
	Subject struct {
		Type string `json:"type"` // "SUBJECT" or "SUBJECT_SET"
		ID   string `json:"id"`
	} `json:"subject"`
	Object struct {
		Type string `json:"type"` // "OBJECT" or "OBJECT_SET"
		ID   string `json:"id"`
	} `json:"object"`
	Action      string                 `json:"action"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Explain     bool                   `json:"explain,omitempty"`
	AtVersionID string                 `json:"atVersionId,omitempty"`
}

type AuthorizationExplain struct {
	SubjectClosure        []uuid.UUID `json:"subjectClosure"`
	ObjectClosure         []uuid.UUID `json:"objectClosure"`
	AllowHits             []uuid.UUID `json:"allowHits"`
	DenyHits              []uuid.UUID `json:"denyHits"`
	EffectiveAllowSetSize int         `json:"effectiveAllowSetSize"`
	EffectiveDenySetSize  int         `json:"effectiveDenySetSize"`
}

// MatchedRule represents a rule that matched during evaluation
type MatchedRule struct {
	RuleID     string  `json:"ruleId"`
	RuleName   string  `json:"ruleName"`
	Effect     string  `json:"effect"`
	ScopeID    *string `json:"scopeId,omitempty"`
	MatchedVia *string `json:"matchedVia,omitempty"`
}

// DenyRule represents a deny rule that matched
type DenyRule struct {
	RuleID   string  `json:"ruleId"`
	RuleName string  `json:"ruleName"`
	Reason   *string `json:"reason,omitempty"`
}

// ExplainTrace provides detailed explanation of evaluation
type ExplainTrace struct {
	Summary struct {
		MatchedRulesCount int    `json:"matchedRulesCount"`
		DenyRulesCount    int    `json:"denyRulesCount"`
		EffectiveDecision string `json:"effectiveDecision"` // "ALLOW" or "DENY"
		VersionID         string `json:"versionId"`
	} `json:"summary"`
	MatchedRules []MatchedRule `json:"matchedRules"`
	Denies       []DenyRule    `json:"denies"`
	Notes        []string      `json:"notes,omitempty"`
}

// EvaluateResponse matches UI expectations
type EvaluateResponse struct {
	Decision    string        `json:"decision"`  // "ALLOW" or "DENY"
	VersionID   string        `json:"versionId"` // Revision as string
	EvaluatedAt time.Time     `json:"evaluatedAt"`
	Trace       *ExplainTrace `json:"trace,omitempty"`
}

type ExplainResponse struct {
	Allowed  bool  `json:"allowed"`
	Revision int64 `json:"revision"`
	Explain  struct {
		SubjectClosure        []uuid.UUID `json:"subject_closure"`
		ObjectClosure         []uuid.UUID `json:"object_closure"`
		AllowHits             []uuid.UUID `json:"allow_hits"`
		DenyHits              []uuid.UUID `json:"deny_hits"`
		EffectiveAllowSetSize int         `json:"effective_allow_set_size"`
		EffectiveDenySetSize  int         `json:"effective_deny_set_size"`
	} `json:"explain"`
}

// --- Graph ---
type GraphSummaryResponse struct {
	Subjects      int `json:"subjects"`
	Objects       int `json:"objects"`
	SubjectSets   int `json:"subject_sets"`
	ObjectSets    int `json:"object_sets"`
	Relationships int `json:"relationships"`
	Rules         int `json:"rules"`
	Denies        int `json:"denies"`
}

type GraphNode struct {
	ID   uuid.UUID `json:"id"`
	Type string    `json:"type"` // "subject", "subject-set", "object", "object-set"
	Name string    `json:"name,omitempty"`
}

type GraphEdge struct {
	From NodeRef `json:"from"`
	To   NodeRef `json:"to"`
	Kind string  `json:"kind"`
}

type GraphNeighborhoodResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphSearchResponse struct {
	Nodes []GraphNode `json:"nodes"`
}

// --- Import/Export ---
type PolicyBundle struct {
	Revision      int64          `json:"revision"`
	Subjects      []Subject      `json:"subjects"`
	SubjectSets   []SubjectSet   `json:"subject_sets"`
	Objects       []Object       `json:"objects"`
	ObjectSets    []ObjectSet    `json:"object_sets"`
	Relationships []Relationship `json:"relationships"`
	Rules         []Rule         `json:"rules"`
	Denies        []Deny         `json:"denies"`
}

type ImportPolicyRequest struct {
	Mode   string       `json:"mode"` // "merge" or "replace"
	Bundle PolicyBundle `json:"bundle"`
}

type ImportPolicyResponse struct {
	Revision int64 `json:"revision"`
	Applied  int   `json:"applied"` // number of changes applied
}
