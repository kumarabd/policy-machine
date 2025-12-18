package server

// Re-export API models for backward compatibility
// All models have been moved to pkg/api/models.go
// This file re-exports them so existing code continues to work

import "github.com/kumarabd/policy-machine/pkg/api"

// Re-export all types from api package
type ErrorResponse = api.ErrorResponse
type MetaResponse = api.MetaResponse
type RevisionResponse = api.RevisionResponse
type PolicyChangeItem = api.PolicyChangeItem
type ChangesResponse = api.ChangesResponse
type Subject = api.Subject
type CreateSubjectRequest = api.CreateSubjectRequest
type UpdateSubjectRequest = api.UpdateSubjectRequest
type SubjectResponse = api.SubjectResponse
type ListSubjectsResponse = api.ListSubjectsResponse
type SubjectGroup = api.SubjectGroup
type CreateSubjectGroupRequest = api.CreateSubjectGroupRequest
type UpdateSubjectGroupRequest = api.UpdateSubjectGroupRequest
type SubjectGroupResponse = api.SubjectGroupResponse
type ListSubjectGroupsResponse = api.ListSubjectGroupsResponse
type Object = api.Object
type CreateObjectRequest = api.CreateObjectRequest
type UpdateObjectRequest = api.UpdateObjectRequest
type ObjectResponse = api.ObjectResponse
type ListObjectsResponse = api.ListObjectsResponse
type ObjectGroup = api.ObjectGroup
type CreateObjectGroupRequest = api.CreateObjectGroupRequest
type UpdateObjectGroupRequest = api.UpdateObjectGroupRequest
type ObjectGroupResponse = api.ObjectGroupResponse
type ListObjectGroupsResponse = api.ListObjectGroupsResponse
type NodeRef = api.NodeRef
type Relationship = api.Relationship
type CreateRelationshipRequest = api.CreateRelationshipRequest
type DeleteRelationshipRequest = api.DeleteRelationshipRequest
type RelationshipResponse = api.RelationshipResponse
type ListRelationshipsResponse = api.ListRelationshipsResponse
type Scope = api.Scope
type Rule = api.Rule
type CreateRuleRequest = api.CreateRuleRequest
type UpdateRuleRequest = api.UpdateRuleRequest
type RuleResponse = api.RuleResponse
type ListRulesResponse = api.ListRulesResponse
type Deny = api.Deny
type CreateDenyRequest = api.CreateDenyRequest
type UpdateDenyRequest = api.UpdateDenyRequest
type DenyResponse = api.DenyResponse
type ListDeniesResponse = api.ListDeniesResponse
type AuthorizeRequest = api.AuthorizeRequest
type AuthorizeResponse = api.AuthorizeResponse
type ExplainResponse = api.ExplainResponse
type GraphSummaryResponse = api.GraphSummaryResponse
type GraphNode = api.GraphNode
type GraphEdge = api.GraphEdge
type GraphNeighborhoodResponse = api.GraphNeighborhoodResponse
type GraphSearchResponse = api.GraphSearchResponse
type PolicyBundle = api.PolicyBundle
type ImportPolicyRequest = api.ImportPolicyRequest
type ImportPolicyResponse = api.ImportPolicyResponse
