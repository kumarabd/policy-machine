package controlplane

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/api/mapper"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
	"gorm.io/datatypes"
)

// ExportPolicy exports the entire policy as a bundle
func (s *Server) ExportPolicy(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	// Get all entities
	subjects, _, _, _ := s.engine.GetDB().ListSubjects(r.Context(), tenantID, "", 10000, "", nil)
	uas, _, _, _ := s.engine.GetDB().ListSubjectSets(r.Context(), tenantID, "", 10000, "")
	objects, _, _, _ := s.engine.GetDB().ListObjects(r.Context(), tenantID, "", 10000, "", nil)
	oas, _, _, _ := s.engine.GetDB().ListObjectSets(r.Context(), tenantID, "", 10000, "")
	edges, _, _, _ := s.engine.GetDB().ListRelationships(r.Context(), tenantID, map[string]interface{}{}, 10000, "")
	_, assocs, _, _, _ := s.engine.GetDB().ListRules(r.Context(), tenantID, map[string]interface{}{}, 10000, "")
	_, prohs, _, _, _ := s.engine.GetDB().ListDenies(r.Context(), tenantID, map[string]interface{}{}, 10000, "")

	// Convert to API models via mapper
	subjectModels := make([]api.Subject, len(subjects))
	for i := range subjects {
		subjectModels[i] = mapper.Subject(&subjects[i])
	}

	subjectAttributes := make([]api.SubjectAttribute, len(uas))
	for i := range uas {
		subjectAttributes[i] = mapper.SubjectAttribute(&uas[i])
	}

	objs := make([]api.Object, len(objects))
	for i := range objects {
		objs[i] = mapper.Object(&objects[i])
	}

	objectAttributes := make([]api.ObjectAttribute, len(oas))
	for i := range oas {
		objectAttributes[i] = mapper.ObjectAttribute(&oas[i])
	}

	relationships := make([]httputil.Relationship, len(edges))
	for i, edge := range edges {
		relationships[i] = httputil.Relationship{
			ID:   edge.ID,
			Kind: InferRelationshipKind(edge.ChildType, edge.ParentType),
			From: httputil.NodeRef{
				Type: MapNodeTypeToUI(edge.ChildType),
				ID:   edge.ChildID,
			},
			To: httputil.NodeRef{
				Type: MapNodeTypeToUI(edge.ParentType),
				ID:   edge.ParentID,
			},
		}
	}

	rules := make([]api.Rule, len(assocs))
	for i, res := range assocs {
		ops, _ := res["operations"].([]string)

		// Get subject type and ID
		subjectType, ok := res["subject_type"].(string)
		if !ok {
			continue // Skip invalid entries
		}
		subjectID, ok := res["subject_id"].(uuid.UUID)
		if !ok {
			continue // Skip invalid entries
		}

		// Get object type and ID
		objectType, ok := res["object_type"].(string)
		if !ok {
			continue // Skip invalid entries
		}
		objectID, ok := res["object_id"].(uuid.UUID)
		if !ok {
			continue // Skip invalid entries
		}

		createdAt := time.Now()
		if ct, ok := res["created_at"].(time.Time); ok {
			createdAt = ct
		}

		rules[i] = api.Rule{
			ID:          res["id"].(uuid.UUID),
			Description: "",
			Actions:     ops,
			SubjectSelector: api.NodeRef{
				Type: subjectType,
				ID:   subjectID,
			},
			ObjectSelector: api.NodeRef{
				Type: objectType,
				ID:   objectID,
			},
			Effect:    "ALLOW",
			Enabled:   true,
			CreatedAt: createdAt,
		}
	}

	denies := make([]httputil.Deny, len(prohs))
	for i, res := range prohs {
		ops, _ := res["operations"].([]string)
		subjectType := "subject"
		if res["subject_type"].(postgres.ProhibitionSubjectType) == postgres.ProhibitUA {
			subjectType = "subject-attribute"
		}
		denies[i] = httputil.Deny{
			ID: res["id"].(uuid.UUID),
			Subject: httputil.NodeRef{
				Type: subjectType,
				ID:   res["subject_id"].(uuid.UUID),
			},
			Operations: ops,
			Targets: []httputil.Scope{
				{
					Type: "object-attribute",
					ID:   res["oa_id"].(uuid.UUID),
				},
			},
		}
	}

	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	bundle := api.PolicyBundle{
		Revision:           rev,
		Subjects:           subjectModels,
		SubjectAttributes:  subjectAttributes,
		Objects:            objs,
		ObjectAttributes:   objectAttributes,
		Relationships:      relationships,
		Rules:              rules,
		Denies:             denies,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

// ImportPolicy imports a policy bundle
func (s *Server) ImportPolicy(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "merge"
	}

	var req httputil.ImportPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	applied := 0

	// Import subjects
	for _, subj := range req.Bundle.Subjects {
		name := subj.Name
		if name == "" {
			name = "imported"
		}
		metaJSON := []byte("{}")
		if len(subj.Metadata) > 0 {
			metaJSON, _ = json.Marshal(subj.Metadata)
		}
		subject := &postgres.Subject{
			ExternalID:  name,
			Display:     name,
			DisplayName: name,
			Kind:        subj.Kind,
			Tags:        datatypes.JSON(metaJSON),
		}
		_, err := s.engine.GetDB().CreateSubject(r.Context(), tenantID, subject)
		if err == nil {
			applied++
		}
	}

	// Import subject attributes
	for _, sa := range req.Bundle.SubjectAttributes {
		ua := &postgres.SubjectAttribute{Name: sa.Name}
		_, err := s.engine.GetDB().CreateSubjectSet(r.Context(), tenantID, ua)
		if err == nil {
			applied++
		}
	}

	// Import objects
	for _, obj := range req.Bundle.Objects {
		name := obj.Name
		if name == "" {
			name = "imported"
		}
		kind := obj.Kind
		if kind == "" {
			kind = "resource"
		}
		metaJSON := []byte("{}")
		if len(obj.Metadata) > 0 {
			metaJSON, _ = json.Marshal(obj.Metadata)
		}
		o := &postgres.Object{
			ExternalID:    name,
			AttributeType: kind,
			DisplayName:   name,
			Kind:          kind,
			Tags:          datatypes.JSON(metaJSON),
		}
		_, err := s.engine.GetDB().CreateObject(r.Context(), tenantID, o)
		if err == nil {
			applied++
		}
	}

	// Import object attributes
	for _, oa := range req.Bundle.ObjectAttributes {
		objAttr := &postgres.ObjectAttribute{Name: oa.Name}
		_, err := s.engine.GetDB().CreateObjectSet(r.Context(), tenantID, objAttr)
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
		_, err = s.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge)
		if err == nil {
			applied++
		}
	}

	// Import rules
	for _, rule := range req.Bundle.Rules {
		_, _, err := s.engine.GetDB().CreateRule(r.Context(), tenantID, rule.SubjectSelector.Type, rule.SubjectSelector.ID, rule.ObjectSelector.Type, rule.ObjectSelector.ID, rule.Actions)
		if err == nil {
			applied++
		}
	}

	// Import denies
	for _, deny := range req.Bundle.Denies {
		subjectType := postgres.ProhibitSubject
		if deny.Subject.Type == "subject-attribute" {
			subjectType = postgres.ProhibitUA
		}
		if len(deny.Targets) > 0 {
			_, _, err := s.engine.GetDB().CreateDeny(r.Context(), tenantID, subjectType, deny.Subject.ID, deny.Targets[0].ID, deny.Operations)
			if err == nil {
				applied++
			}
		}
	}

	rev, _ := s.engine.GetDB().GetCurrentRevision(r.Context(), tenantID)

	response := httputil.ImportPolicyResponse{
		Revision: rev,
		Applied:  applied,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions use the ones from relationships_handlers.go
