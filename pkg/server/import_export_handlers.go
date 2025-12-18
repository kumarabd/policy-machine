package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/mock"
	"github.com/kumarabd/policy-machine/pkg/postgres"
)

// ExportPolicy exports the entire policy as a bundle
func (h *BaseServer) ExportPolicy(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ExportPolicy(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	// Get all entities
	users, _, _, _ := h.engine.GetDB().ListSubjects(r.Context(), tenantID, "", 10000, "")
	uas, _, _, _ := h.engine.GetDB().ListSubjectGroups(r.Context(), tenantID, "", 10000, "")
	objects, _, _, _ := h.engine.GetDB().ListObjects(r.Context(), tenantID, "", 10000, "")
	oas, _, _, _ := h.engine.GetDB().ListObjectGroups(r.Context(), tenantID, "", 10000, "")
	edges, _, _, _ := h.engine.GetDB().ListRelationships(r.Context(), tenantID, map[string]interface{}{}, 10000, "")
	_, assocs, _, _, _ := h.engine.GetDB().ListRules(r.Context(), tenantID, map[string]interface{}{}, 10000, "")
	_, prohs, _, _, _ := h.engine.GetDB().ListDenies(r.Context(), tenantID, map[string]interface{}{}, 10000, "")

	// Convert to API models
	subjects := make([]Subject, len(users))
	for i, u := range users {
		subjects[i] = Subject{
			ID:         u.ID,
			ExternalID: u.ExternalID,
			Email:      u.Email,
			Display:    u.Display,
			CreatedAt:  u.CreatedAt,
		}
	}

	subjectGroups := make([]SubjectGroup, len(uas))
	for i, ua := range uas {
		subjectGroups[i] = SubjectGroup{
			ID:        ua.ID,
			Name:      ua.Name,
			CreatedAt: ua.CreatedAt,
		}
	}

	objs := make([]Object, len(objects))
	for i, o := range objects {
		objs[i] = Object{
			ID:         o.ID,
			ExternalID: o.ExternalID,
			Type:       o.Type,
			CreatedAt:  o.CreatedAt,
		}
	}

	objectGroups := make([]ObjectGroup, len(oas))
	for i, oa := range oas {
		objectGroups[i] = ObjectGroup{
			ID:        oa.ID,
			Name:      oa.Name,
			CreatedAt: oa.CreatedAt,
		}
	}

	relationships := make([]Relationship, len(edges))
	for i, edge := range edges {
		relationships[i] = Relationship{
			ID:   edge.ID,
			Kind: InferRelationshipKind(edge.ChildType, edge.ParentType),
			From: NodeRef{
				Type: MapNodeTypeToUI(edge.ChildType),
				ID:   edge.ChildID,
			},
			To: NodeRef{
				Type: MapNodeTypeToUI(edge.ParentType),
				ID:   edge.ParentID,
			},
		}
	}

	rules := make([]api.Rule, len(assocs))
	for i, res := range assocs {
		ops, _ := res["operations"].([]string)
		uaID := res["ua_id"].(uuid.UUID)
		oaID := res["oa_id"].(uuid.UUID)
		rules[i] = api.Rule{
			ID:          res["id"].(uuid.UUID),
			Name:        "Rule", // TODO: Get from DB if available
			Description: "",
			Actions:     ops,
			SubjectSelector: api.NodeRef{
				Type: "subject-set",
				ID:   uaID,
			},
			ObjectSelector: api.NodeRef{
				Type: "object-set",
				ID:   oaID,
			},
			Effect:    "ALLOW",
			Enabled:   true,
			CreatedAt: time.Now(), // TODO: Get from DB if available
		}
	}

	denies := make([]Deny, len(prohs))
	for i, res := range prohs {
		ops, _ := res["operations"].([]string)
		subjectType := "subject"
		if res["subject_type"].(postgres.ProhibitionSubjectType) == postgres.ProhibitUA {
			subjectType = "subject-set"
		}
		denies[i] = Deny{
			ID: res["id"].(uuid.UUID),
			Subject: NodeRef{
				Type: subjectType,
				ID:   res["subject_id"].(uuid.UUID),
			},
			Operations: ops,
			Targets: []Scope{
				{
					Type: "object-set",
					ID:   res["oa_id"].(uuid.UUID),
				},
			},
		}
	}

	rev, _ := h.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	bundle := PolicyBundle{
		Revision:      rev,
		Subjects:      subjects,
		SubjectSets:   subjectGroups,
		Objects:       objs,
		ObjectSets:    objectGroups,
		Relationships: relationships,
		Rules:         rules,
		Denies:        denies,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

// ImportPolicy imports a policy bundle
func (h *BaseServer) ImportPolicy(w http.ResponseWriter, r *http.Request) {
	if IsMockMode(r.Context()) {
		mock.ImportPolicy(w, r)
		return
	}

	tenantID, ok := GetTenantID(r.Context())
	if !ok {
		respondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "merge"
	}

	var req ImportPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	applied := 0

	// Import subjects
	for _, subj := range req.Bundle.Subjects {
		user := &postgres.User{
			ExternalID: subj.ExternalID,
			Email:      subj.Email,
			Display:    subj.Display,
		}
		_, err := h.engine.GetDB().CreateSubject(r.Context(), tenantID, user)
		if err == nil {
			applied++
		}
	}

	// Import subject sets
	for _, sg := range req.Bundle.SubjectSets {
		ua := &postgres.UserAttribute{Name: sg.Name}
		_, err := h.engine.GetDB().CreateSubjectGroup(r.Context(), tenantID, ua)
		if err == nil {
			applied++
		}
	}

	// Import objects
	for _, obj := range req.Bundle.Objects {
		o := &postgres.Object{
			ExternalID: obj.ExternalID,
			Type:       obj.Type,
		}
		_, err := h.engine.GetDB().CreateObject(r.Context(), tenantID, o)
		if err == nil {
			applied++
		}
	}

	// Import object sets
	for _, og := range req.Bundle.ObjectSets {
		oa := &postgres.ObjectAttribute{Name: og.Name}
		_, err := h.engine.GetDB().CreateObjectGroup(r.Context(), tenantID, oa)
		if err == nil {
			applied++
		}
	}

	// Import relationships
	for _, rel := range req.Bundle.Relationships {
		childType, parentType, err := postgres.MapRelationshipKindToEdgeTypes(rel.Kind, rel.From.Type, rel.To.Type)
		if err != nil {
			continue
		}
		edge := &postgres.AssignmentEdge{
			ChildType:  childType,
			ChildID:    rel.From.ID,
			ParentType: parentType,
			ParentID:   rel.To.ID,
		}
		_, err = h.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge)
		if err == nil {
			applied++
		}
	}

	// Import rules
	for _, rule := range req.Bundle.Rules {
		_, _, err := h.engine.GetDB().CreateRule(r.Context(), tenantID, rule.SubjectSelector.ID, rule.ObjectSelector.ID, rule.Actions)
		if err == nil {
			applied++
		}
	}

	// Import denies
	for _, deny := range req.Bundle.Denies {
		subjectType := postgres.ProhibitUser
		if deny.Subject.Type == "subject-set" {
			subjectType = postgres.ProhibitUA
		}
		if len(deny.Targets) > 0 {
			_, _, err := h.engine.GetDB().CreateDeny(r.Context(), tenantID, subjectType, deny.Subject.ID, deny.Targets[0].ID, deny.Operations)
			if err == nil {
				applied++
			}
		}
	}

	rev, _ := h.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	response := ImportPolicyResponse{
		Revision: rev,
		Applied:  applied,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions use the ones from relationships_handlers.go

