package mapper

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// Postgres -> API (persistence -> API contract)
// API Subject/Object use: id, name, kind, metadata.
// Postgres still uses ExternalID, DisplayName, Kind, Tags; we map name <-> ExternalID, metadata <-> Tags.

// Subject converts postgres.Subject to api.Subject
func Subject(db *postgres.Subject) api.Subject {
	return api.Subject{
		ID:       db.ID,
		Name:     db.ExternalID,
		Kind:     db.Kind,
		Metadata: jsonToMapStringString(db.Tags),
	}
}

// Object converts postgres.Object to api.Object
func Object(db *postgres.Object) api.Object {
	return api.Object{
		ID:       db.ID,
		Name:     db.ExternalID,
		Kind:     db.Kind,
		Metadata: jsonToMapStringString(db.Tags),
	}
}

// SubjectAttribute converts postgres.SubjectAttribute to api.SubjectAttribute
func SubjectAttribute(db *postgres.SubjectAttribute) api.SubjectAttribute {
	return api.SubjectAttribute{
		ID:       db.ID,
		Name:     db.Name,
		Metadata: nil, // postgres has no metadata column for attributes yet
	}
}

// ObjectAttribute converts postgres.ObjectAttribute to api.ObjectAttribute
func ObjectAttribute(db *postgres.ObjectAttribute) api.ObjectAttribute {
	return api.ObjectAttribute{
		ID:       db.ID,
		Name:     db.Name,
		Metadata: nil,
	}
}

func jsonToMapStringString(tags []byte) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(tags, &m); err != nil {
		// try as string array and return nil for now
		return nil
	}
	return m
}

func mapStringStringToJSON(m map[string]string) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}
	b, _ := json.Marshal(m)
	return b
}

// API -> Postgres (for create/update)

// CreateSubjectRequestToPostgres builds postgres.Subject from API request
func CreateSubjectRequestToPostgres(req api.CreateSubjectRequest) *postgres.Subject {
	name := req.Name
	if name == "" {
		name = "unknown"
	}
	return &postgres.Subject{
		ExternalID:  name,
		Display:     name,
		DisplayName: name,
		Kind:        req.Kind,
		Tags:        mapStringStringToJSON(req.Metadata),
	}
}

// CreateObjectRequestToPostgres builds postgres.Object from API request
func CreateObjectRequestToPostgres(req api.CreateObjectRequest) *postgres.Object {
	name := req.Name
	if name == "" {
		name = "unknown"
	}
	kind := req.Kind
	if kind == "" {
		kind = "resource"
	}

	// Derive a human-friendly display name from the canonical name.
	// For k8s URIs like k8s://cluster/ns/nsName/.../resourceName,
	// we take the last path segment as the display name.
	displayName := name
	if strings.HasPrefix(name, "k8s://") {
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			if last != "" {
				displayName = last
			}
		}
	}

	return &postgres.Object{
		ExternalID:    name,
		DisplayName:   displayName,
		Kind:          kind,
		AttributeType: kind,
		Tags:          mapStringStringToJSON(req.Metadata),
	}
}

// CreateSubjectAttributeRequestToPostgres builds postgres.SubjectAttribute from API request
func CreateSubjectAttributeRequestToPostgres(req api.CreateSubjectAttributeRequest) *postgres.SubjectAttribute {
	return &postgres.SubjectAttribute{
		Name:          req.Name,
		AttributeType: postgres.AttributeTypeCustom,
	}
}

// CreateObjectAttributeRequestToPostgres builds postgres.ObjectAttribute from API request
func CreateObjectAttributeRequestToPostgres(req api.CreateObjectAttributeRequest) *postgres.ObjectAttribute {
	return &postgres.ObjectAttribute{
		Name:          req.Name,
		AttributeType: postgres.AttributeTypeCustom,
	}
}

// NodeRef builds api.NodeRef from type and id
func NodeRef(typ string, id uuid.UUID) api.NodeRef {
	return api.NodeRef{Type: typ, ID: id}
}
