package http

// Re-export API types for convenience
// All types are from pkg/api package
import "github.com/kumarabd/policy-machine/pkg/api"

// Re-export commonly used types
type (
	// Request/Response types
	AuthorizeRequest        = api.AuthorizeRequest
	AuthorizeResponse       = api.AuthorizeResponse
	EvaluateRequest         = api.EvaluateRequest
	EvaluateResponse        = api.EvaluateResponse
	AuthorizationExplain    = api.AuthorizationExplain
	CreateSubjectRequest    = api.CreateSubjectRequest
	UpdateSubjectRequest    = api.UpdateSubjectRequest
	SubjectResponse         = api.SubjectResponse
	CreateObjectRequest     = api.CreateObjectRequest
	UpdateObjectRequest     = api.UpdateObjectRequest
	ObjectResponse          = api.ObjectResponse
	CreateSubjectSetRequest = api.CreateSubjectGroupRequest
	UpdateSubjectSetRequest = api.UpdateSubjectGroupRequest
	SubjectSetResponse    = api.SubjectGroupResponse
	CreateObjectSetRequest = api.CreateObjectGroupRequest
	UpdateObjectSetRequest = api.UpdateObjectGroupRequest
	ObjectSetResponse     = api.ObjectGroupResponse
	
	// Also provide Group aliases for backward compatibility with DB methods
	CreateSubjectGroupRequest = api.CreateSubjectGroupRequest
	UpdateSubjectGroupRequest = api.UpdateSubjectGroupRequest
	SubjectGroupResponse    = api.SubjectGroupResponse
	CreateObjectGroupRequest = api.CreateObjectGroupRequest
	UpdateObjectGroupRequest = api.UpdateObjectGroupRequest
	ObjectGroupResponse     = api.ObjectGroupResponse
	CreateRelationshipRequest = api.CreateRelationshipRequest
	DeleteRelationshipRequest = api.DeleteRelationshipRequest
	RelationshipResponse    = api.RelationshipResponse
	CreateRuleRequest       = api.CreateRuleRequest
	UpdateRuleRequest       = api.UpdateRuleRequest
	RuleResponse            = api.RuleResponse
	CreateDenyRequest        = api.CreateDenyRequest
	UpdateDenyRequest        = api.UpdateDenyRequest
	DenyResponse            = api.DenyResponse
	ImportPolicyRequest      = api.ImportPolicyRequest
	ImportPolicyResponse     = api.ImportPolicyResponse

	// Entity types
	Subject                 = api.Subject
	Object                  = api.Object
	SubjectSet              = api.SubjectSet
	ObjectSet               = api.ObjectSet
	Relationship            = api.Relationship
	Rule                    = api.Rule
	Deny                    = api.Deny
	NodeRef                 = api.NodeRef
	Scope                   = api.Scope
	PolicyScope             = api.PolicyScope
	PolicyBundle            = api.PolicyBundle

	// Response types
	ListSubjectsResponse     = api.ListSubjectsResponse
	ListObjectsResponse      = api.ListObjectsResponse
	ListSubjectSetsResponse = api.ListSubjectGroupsResponse
	ListObjectSetsResponse = api.ListObjectGroupsResponse
	ListRelationshipsResponse = api.ListRelationshipsResponse
	ListRulesResponse        = api.ListRulesResponse
	ListDeniesResponse        = api.ListDeniesResponse
	GraphSummaryResponse     = api.GraphSummaryResponse
	GraphNeighborhoodResponse = api.GraphNeighborhoodResponse
	GraphNode                = api.GraphNode
	GraphEdge                = api.GraphEdge
	GraphSearchResponse      = api.GraphSearchResponse
	ExplainResponse            = api.ExplainResponse
)

