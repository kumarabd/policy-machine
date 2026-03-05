// Package api defines the HTTP API contract for the policy machine.
// Models are client-agnostic DTOs with camelCase JSON for broad compatibility.
// Persistence types live in internal/postgres; conversion is in internal/api/mapper.
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
	ServiceVersion string    `json:"serviceVersion"`
	Revision       int64     `json:"revision"`
	AppliedSeq     int64     `json:"appliedSeq"`
	Now            time.Time `json:"now"`
}

type RevisionResponse struct {
	Revision int64 `json:"revision"`
}

type PolicyChangeItem struct {
	Seq       int64                  `json:"seq"`
	Revision  int64                  `json:"revision"`
	Kind      string                 `json:"kind"`
	Op        string                 `json:"op"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

// AttributeNode represents an attribute in the attribute graph
type AttributeNode struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	AttributeType string    `json:"type"` // "native" or "custom"
}

// AttributeEdge represents a relationship between two attributes (child -> parent)
type AttributeEdge struct {
	ChildID  uuid.UUID `json:"child_id"`
	ParentID uuid.UUID `json:"parent_id"`
}

// AttributeSubgraphResponse represents the complete attribute subgraph for a subject or object
type AttributeSubgraphResponse struct {
	Nodes []AttributeNode `json:"nodes"`
	Edges []AttributeEdge `json:"edges"`
}

type ChangesResponse struct {
	FromSeq  int64              `json:"fromSeq"`
	ToSeq    int64              `json:"toSeq"`
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

// --- Subjects ---
type Subject struct {
	ID       uuid.UUID         `json:"id"`
	Name     string            `json:"name"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CreateSubjectRequest struct {
	Name     string            `json:"name"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type UpdateSubjectRequest struct {
	Name     string            `json:"name,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type SubjectResponse struct {
	Subject  Subject `json:"subject"`
	Revision int64   `json:"revision"`
}

type ListSubjectsResponse struct {
	Subjects []Subject `json:"subjects"`
	Cursor   string    `json:"cursor,omitempty"`
	HasMore  bool      `json:"hasMore"`
}

// --- Subject Attributes ---
type SubjectAttribute struct {
	ID       uuid.UUID         `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CreateSubjectAttributeRequest struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
	ParentName string          `json:"parentName,omitempty"`
	ParentID   *uuid.UUID      `json:"parentId,omitempty"`
}

type UpdateSubjectAttributeRequest struct {
	Name     string            `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type SubjectAttributeResponse struct {
	Attribute SubjectAttribute `json:"attribute"`
	Revision  int64            `json:"revision"`
}

type ListSubjectAttributesResponse struct {
	Attributes []SubjectAttribute `json:"attributes"`
	Cursor     string             `json:"cursor,omitempty"`
	HasMore    bool               `json:"hasMore"`
}

// --- Objects ---
type Object struct {
	ID       uuid.UUID         `json:"id"`
	Name     string            `json:"name"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CreateObjectRequest struct {
	Name     string            `json:"name"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type UpdateObjectRequest struct {
	Name     string            `json:"name,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ObjectResponse struct {
	Object   Object `json:"object"`
	Revision int64  `json:"revision"`
}

type ListObjectsResponse struct {
	Objects []Object `json:"objects"`
	Cursor  string   `json:"cursor,omitempty"`
	HasMore bool     `json:"hasMore"`
}

// --- Object Attributes ---
type ObjectAttribute struct {
	ID       uuid.UUID         `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CreateObjectAttributeRequest struct {
	Name       string            `json:"name"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	ParentName string            `json:"parentName,omitempty"`
	ParentID   *uuid.UUID        `json:"parentId,omitempty"`
}

type UpdateObjectAttributeRequest struct {
	Name     string            `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ObjectAttributeResponse struct {
	Attribute ObjectAttribute `json:"attribute"`
	Revision  int64           `json:"revision"`
}

type ListObjectAttributesResponse struct {
	Attributes []ObjectAttribute `json:"attributes"`
	Cursor     string            `json:"cursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
}

// --- Relationships (Assignment Edges) ---
type NodeRef struct {
	Type string    `json:"type"` // "subject", "subject-attribute", "object", "object-attribute"
	ID   uuid.UUID `json:"id"`
}

type Relationship struct {
	ID   uuid.UUID `json:"id,omitempty"`
	Kind string    `json:"kind"` // "subject_member_of_attribute", "subject_attribute_parent_of_attribute", etc.
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
	HasMore       bool           `json:"hasMore"`
}

// --- Rules (Associations) ---
// Scope is used in Deny targets.
// For Deny targets, use "subject-attribute" or "object-attribute" to refer to attribute entities.
type Scope struct {
	Type string    `json:"type"` // "subject-attribute", "object-attribute", "subject", "object"
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
	HasMore bool   `json:"hasMore"`
}

// --- Denies (Prohibitions) ---
// In Deny API, use "subject-attribute" and "object-attribute" to refer to attribute entities.
type Deny struct {
	ID          uuid.UUID `json:"id"`
	Subject     NodeRef   `json:"subject"` // type: "subject" or "subject-attribute"
	Operations  []string  `json:"operations"`
	Targets     []Scope   `json:"targets"` // type: "object" or "object-attribute"
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
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
	HasMore bool   `json:"hasMore"`
}

// --- Authorization ---
type AuthorizeRequest struct {
	SubjectID uuid.UUID `json:"subject_id"`
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
		Type string `json:"type"` // "SUBJECT" or "SUBJECT_ATTRIBUTE"
		ID   string `json:"id"`
	} `json:"subject"`
	Object struct {
		Type string `json:"type"` // "OBJECT" or "OBJECT_ATTRIBUTE"
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
	SubjectAttributes int `json:"subject_attributes"`
	ObjectAttributes  int `json:"object_attributes"`
	Relationships int `json:"relationships"`
	Rules         int `json:"rules"`
	Denies        int `json:"denies"`
}

type GraphNode struct {
	ID   uuid.UUID `json:"id"`
	Type string    `json:"type"` // "subject", "subject-attribute", "object", "object-attribute"
	Name string `json:"name,omitempty"`
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
	SubjectAttributes []SubjectAttribute `json:"subject_attributes"`
	Objects           []Object           `json:"objects"`
	ObjectAttributes  []ObjectAttribute  `json:"object_attributes"`
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
