package api

import (
	"database/sql"

	"github.com/kumarabd/policy-machine/model"
)

type DataHandler interface {
	Ping() (string, error)
	AddResource(*model.Resource) error
	AddResourceBulk([]*model.Resource) error
	FetchResource(*model.Resource, bool) error
	FetchResourceBulk(bool) ([]*model.Resource, error)

	AddAttribute(*model.Attribute) error
	AddAttributeBulk([]*model.Attribute) error
	FetchAttribute(*model.Attribute, bool) error
	FetchOrCreateAttribute(*model.Attribute, bool) error
	FetchAttributeBulk(bool) ([]*model.Attribute, error)

	AddSubject(*model.Subject) error
	AddSubjectIfDoesntExist(*model.Subject) error
	AddSubjectBulk([]*model.Subject) error
	FetchSubject(*model.Subject, bool) error
	FetchOrCreateSubject(*model.Subject, bool) error
	FetchSubjectBulk(bool) ([]*model.Subject, error)

	AddAssignment(*model.Assignment) error
	AddAssignmentBulk([]*model.Assignment) error
	FetchAssignment(*model.Assignment, bool) error
	FetchAssignmentBulk(bool) ([]*model.Assignment, error)

	AddAssociation(*model.Association) error
	AddAssociationBulk([]*model.Association) error
	FetchAssociation(*model.Association, bool) error
	FetchAssociationBulk(bool) ([]*model.Association, error)
	AddActionsToAssociations(*model.Association, []string) error
	AddActionsToProhibitions(*model.Prohibition, []string) error

	AddPolicy(*model.Policy) error
	AddPolicyBulk([]*model.Policy) error
	FetchPolicy(*model.Policy, bool) error
	FetchPolicyBulk(bool) ([]*model.Policy, error)

	AddPolicyClass(*model.PolicyClass) error
	AddPolicyClassBulk([]*model.PolicyClass) error
	FetchPolicyClass(*model.PolicyClass, bool) error
	FetchPolicyClassBulk(bool) ([]*model.PolicyClass, error)

	FetchEntityForID(string, *model.Entity) error
	FetchRelationshipsForSource(string, *[]model.Relationship) error

	// NGAC-specific operations for prohibitions
	AddProhibition(*model.Prohibition) error
	AddProhibitionBulk([]*model.Prohibition) error
	FetchProhibition(*model.Prohibition, bool) error
	FetchProhibitionBulk(bool) ([]*model.Prohibition, error)
	FetchProhibitionsForPolicyClass(string) ([]*model.Prohibition, error)

	DB() *sql.DB
}
