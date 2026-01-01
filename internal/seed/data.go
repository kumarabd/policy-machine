package seed

import (
	"time"

	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"gorm.io/datatypes"
)

// SeedData contains all seed data for the system
type SeedData struct {
	TenantID string

	// Core entities
	Subjects      []postgres.Subject
	Objects       []postgres.Object
	SubjectSets   []postgres.SubjectAttribute
	ObjectSets    []postgres.ObjectAttribute
	PolicyClasses []postgres.PolicyClass

	// Relationships (AssignmentEdges)
	Relationships []postgres.AssignmentEdge

	// Rules (Associations)
	Associations []AssociationSeed
	Rules        []RuleSeed

	// Denies (Prohibitions)
	Prohibitions []ProhibitionSeed

	// Obligations
	Obligations []postgres.Obligation
}

// AssociationSeed represents an association with its operations
type AssociationSeed struct {
	UAID       uuid.UUID
	OAID       uuid.UUID
	Operations []string
}

// RuleSeed is an alias for AssociationSeed (for clarity)
type RuleSeed = AssociationSeed

// ProhibitionSeed represents a prohibition with its operations
type ProhibitionSeed struct {
	SubjectType postgres.ProhibitionSubjectType
	SubjectID   uuid.UUID
	OAID        uuid.UUID
	Operations  []string
}

// GetSeedData returns comprehensive seed data covering all system scenarios
func GetSeedData(tenantID string) *SeedData {
	now := time.Now()

	// Predefined UUIDs for consistency
	// Subjects
	subjAlice := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	subjBob := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	subjCarol := uuid.MustParse("10000000-0000-0000-0000-000000000003")
	subjDavid := uuid.MustParse("10000000-0000-0000-0000-000000000004")
	subjEve := uuid.MustParse("10000000-0000-0000-0000-000000000005")
	subjFrank := uuid.MustParse("10000000-0000-0000-0000-000000000006")
	subjGrace := uuid.MustParse("10000000-0000-0000-0000-000000000007")
	subjHenry := uuid.MustParse("10000000-0000-0000-0000-000000000008")

	// Subject Sets (SubjectAttributes)
	uaEngineering := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	uaBackend := uuid.MustParse("20000000-0000-0000-0000-000000000002")
	uaFrontend := uuid.MustParse("20000000-0000-0000-0000-000000000003")
	uaManagement := uuid.MustParse("20000000-0000-0000-0000-000000000004")
	uaOperations := uuid.MustParse("20000000-0000-0000-0000-000000000005")
	uaSecurity := uuid.MustParse("20000000-0000-0000-0000-000000000006")
	uaDataTeam := uuid.MustParse("20000000-0000-0000-0000-000000000007")

	// Objects
	objDocPRD := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	objDocAPI := uuid.MustParse("30000000-0000-0000-0000-000000000002")
	objProjectPortal := uuid.MustParse("30000000-0000-0000-0000-000000000003")
	objRepoEngine := uuid.MustParse("30000000-0000-0000-0000-000000000004")
	objSecretDB := uuid.MustParse("30000000-0000-0000-0000-000000000005")
	objAPIManagement := uuid.MustParse("30000000-0000-0000-0000-000000000006")
	objRepoFrontend := uuid.MustParse("30000000-0000-0000-0000-000000000007")
	objSecretAWS := uuid.MustParse("30000000-0000-0000-0000-000000000008")
	objAPIAnalytics := uuid.MustParse("30000000-0000-0000-0000-000000000009")
	objDocSecurity := uuid.MustParse("30000000-0000-0000-0000-000000000010")

	// Object Sets (ObjectAttributes)
	oaPublicDocs := uuid.MustParse("40000000-0000-0000-0000-000000000001")
	oaPrivateDocs := uuid.MustParse("40000000-0000-0000-0000-000000000002")
	oaActiveProjects := uuid.MustParse("40000000-0000-0000-0000-000000000003")
	oaRepositories := uuid.MustParse("40000000-0000-0000-0000-000000000004")
	oaProdSecrets := uuid.MustParse("40000000-0000-0000-0000-000000000005")
	oaPublicAPIs := uuid.MustParse("40000000-0000-0000-0000-000000000006")
	oaInternalAPIs := uuid.MustParse("40000000-0000-0000-0000-000000000007")
	oaSensitiveData := uuid.MustParse("40000000-0000-0000-0000-000000000008")

	// Policy Classes
	pcDefault := uuid.MustParse("50000000-0000-0000-0000-000000000001")
	pcHighSecurity := uuid.MustParse("50000000-0000-0000-0000-000000000002")
	pcPublicAccess := uuid.MustParse("50000000-0000-0000-0000-000000000003")

	return &SeedData{
		TenantID: tenantID,
		Subjects: []postgres.Subject{
			{ID: subjAlice, TenantID: tenantID, ExternalID: "alice.johnson", Email: "alice.johnson@acme.com", Display: "Alice Johnson", CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: subjBob, TenantID: tenantID, ExternalID: "bob.smith", Email: "bob.smith@acme.com", Display: "Bob Smith", CreatedAt: now.Add(-180 * 24 * time.Hour)},
			{ID: subjCarol, TenantID: tenantID, ExternalID: "carol.williams", Email: "carol.williams@acme.com", Display: "Carol Williams", CreatedAt: now.Add(-730 * 24 * time.Hour)},
			{ID: subjDavid, TenantID: tenantID, ExternalID: "david.brown", Email: "david.brown@acme.com", Display: "David Brown", CreatedAt: now.Add(-240 * 24 * time.Hour)},
			{ID: subjEve, TenantID: tenantID, ExternalID: "eve.davis", Email: "eve.davis@acme.com", Display: "Eve Davis", CreatedAt: now.Add(-120 * 24 * time.Hour)},
			{ID: subjFrank, TenantID: tenantID, ExternalID: "frank.miller", Email: "frank.miller@acme.com", Display: "Frank Miller", CreatedAt: now.Add(-600 * 24 * time.Hour)},
			{ID: subjGrace, TenantID: tenantID, ExternalID: "grace.lee", Email: "grace.lee@acme.com", Display: "Grace Lee", CreatedAt: now.Add(-400 * 24 * time.Hour)},
			{ID: subjHenry, TenantID: tenantID, ExternalID: "henry.wilson", Email: "henry.wilson@acme.com", Display: "Henry Wilson", CreatedAt: now.Add(-500 * 24 * time.Hour)},
		},
		SubjectSets: []postgres.SubjectAttribute{
			{ID: uaEngineering, TenantID: tenantID, Name: "Engineering Team", CreatedAt: now.Add(-730 * 24 * time.Hour)},
			{ID: uaBackend, TenantID: tenantID, Name: "Backend Developers", CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uaFrontend, TenantID: tenantID, Name: "Frontend Developers", CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uaManagement, TenantID: tenantID, Name: "Management", CreatedAt: now.Add(-600 * 24 * time.Hour)},
			{ID: uaOperations, TenantID: tenantID, Name: "Operations Team", CreatedAt: now.Add(-450 * 24 * time.Hour)},
			{ID: uaSecurity, TenantID: tenantID, Name: "Security Team", CreatedAt: now.Add(-300 * 24 * time.Hour)},
			{ID: uaDataTeam, TenantID: tenantID, Name: "Data Team", CreatedAt: now.Add(-200 * 24 * time.Hour)},
		},
		Objects: []postgres.Object{
			{ID: objDocPRD, TenantID: tenantID, ExternalID: "doc/prd-customer-portal", Type: "document", CreatedAt: now.Add(-120 * 24 * time.Hour)},
			{ID: objDocAPI, TenantID: tenantID, ExternalID: "doc/api-design-v3", Type: "document", CreatedAt: now.Add(-45 * 24 * time.Hour)},
			{ID: objProjectPortal, TenantID: tenantID, ExternalID: "project/customer-portal-redesign", Type: "project", CreatedAt: now.Add(-60 * 24 * time.Hour)},
			{ID: objRepoEngine, TenantID: tenantID, ExternalID: "repo/policy-engine", Type: "repository", CreatedAt: now.Add(-730 * 24 * time.Hour)},
			{ID: objSecretDB, TenantID: tenantID, ExternalID: "secret/db-prod-credentials", Type: "secret", CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: objAPIManagement, TenantID: tenantID, ExternalID: "api/subject-management-v2", Type: "api", CreatedAt: now.Add(-180 * 24 * time.Hour)},
			{ID: objRepoFrontend, TenantID: tenantID, ExternalID: "repo/frontend-app", Type: "repository", CreatedAt: now.Add(-450 * 24 * time.Hour)},
			{ID: objSecretAWS, TenantID: tenantID, ExternalID: "secret/aws-access-key-prod", Type: "secret", CreatedAt: now.Add(-200 * 24 * time.Hour)},
			{ID: objAPIAnalytics, TenantID: tenantID, ExternalID: "api/analytics-v1", Type: "api", CreatedAt: now.Add(-300 * 24 * time.Hour)},
			{ID: objDocSecurity, TenantID: tenantID, ExternalID: "doc/security-policy", Type: "document", CreatedAt: now.Add(-180 * 24 * time.Hour)},
		},
		ObjectSets: []postgres.ObjectAttribute{
			{ID: oaPublicDocs, TenantID: tenantID, Name: "Public Documents", CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: oaPrivateDocs, TenantID: tenantID, Name: "Private Documents", CreatedAt: now.Add(-300 * 24 * time.Hour)},
			{ID: oaActiveProjects, TenantID: tenantID, Name: "Active Projects", CreatedAt: now.Add(-180 * 24 * time.Hour)},
			{ID: oaRepositories, TenantID: tenantID, Name: "Code Repositories", CreatedAt: now.Add(-600 * 24 * time.Hour)},
			{ID: oaProdSecrets, TenantID: tenantID, Name: "Production Secrets", CreatedAt: now.Add(-200 * 24 * time.Hour)},
			{ID: oaPublicAPIs, TenantID: tenantID, Name: "Public APIs", CreatedAt: now.Add(-150 * 24 * time.Hour)},
			{ID: oaInternalAPIs, TenantID: tenantID, Name: "Internal APIs", CreatedAt: now.Add(-250 * 24 * time.Hour)},
			{ID: oaSensitiveData, TenantID: tenantID, Name: "Sensitive Data", CreatedAt: now.Add(-100 * 24 * time.Hour)},
		},
		PolicyClasses: []postgres.PolicyClass{
			{ID: pcDefault, TenantID: tenantID, Name: "Default", CreatedAt: now.Add(-1000 * 24 * time.Hour)},
			{ID: pcHighSecurity, TenantID: tenantID, Name: "High Security", CreatedAt: now.Add(-500 * 24 * time.Hour)},
			{ID: pcPublicAccess, TenantID: tenantID, Name: "Public Access", CreatedAt: now.Add(-400 * 24 * time.Hour)},
		},
		Relationships: []postgres.AssignmentEdge{
			// Subject -> Subject Set relationships
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000001"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjAlice, ParentType: postgres.NodeUA, ParentID: uaEngineering, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000002"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjBob, ParentType: postgres.NodeUA, ParentID: uaEngineering, CreatedAt: now.Add(-180 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000003"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjAlice, ParentType: postgres.NodeUA, ParentID: uaBackend, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000004"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjBob, ParentType: postgres.NodeUA, ParentID: uaFrontend, CreatedAt: now.Add(-180 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000005"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjCarol, ParentType: postgres.NodeUA, ParentID: uaManagement, CreatedAt: now.Add(-730 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000006"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjDavid, ParentType: postgres.NodeUA, ParentID: uaOperations, CreatedAt: now.Add(-240 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000007"), TenantID: tenantID, ChildType: postgres.NodeSubject, ChildID: subjHenry, ParentType: postgres.NodeUA, ParentID: uaSecurity, CreatedAt: now.Add(-500 * 24 * time.Hour)},

			// Subject Set -> Subject Set (hierarchical)
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000008"), TenantID: tenantID, ChildType: postgres.NodeUA, ChildID: uaBackend, ParentType: postgres.NodeUA, ParentID: uaEngineering, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000009"), TenantID: tenantID, ChildType: postgres.NodeUA, ChildID: uaFrontend, ParentType: postgres.NodeUA, ParentID: uaEngineering, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000010"), TenantID: tenantID, ChildType: postgres.NodeUA, ChildID: uaDataTeam, ParentType: postgres.NodeUA, ParentID: uaEngineering, CreatedAt: now.Add(-200 * 24 * time.Hour)},

			// Object -> Object Set relationships
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000011"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objDocPRD, ParentType: postgres.NodeOA, ParentID: oaPublicDocs, CreatedAt: now.Add(-120 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000012"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objDocAPI, ParentType: postgres.NodeOA, ParentID: oaPrivateDocs, CreatedAt: now.Add(-45 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000013"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objProjectPortal, ParentType: postgres.NodeOA, ParentID: oaActiveProjects, CreatedAt: now.Add(-60 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000014"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objRepoEngine, ParentType: postgres.NodeOA, ParentID: oaRepositories, CreatedAt: now.Add(-730 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000015"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objSecretDB, ParentType: postgres.NodeOA, ParentID: oaProdSecrets, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000016"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objAPIManagement, ParentType: postgres.NodeOA, ParentID: oaPublicAPIs, CreatedAt: now.Add(-180 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000017"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objSecretDB, ParentType: postgres.NodeOA, ParentID: oaSensitiveData, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000018"), TenantID: tenantID, ChildType: postgres.NodeObject, ChildID: objSecretAWS, ParentType: postgres.NodeOA, ParentID: oaSensitiveData, CreatedAt: now.Add(-200 * 24 * time.Hour)},

			// Object Set -> Object Set (hierarchical)
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000019"), TenantID: tenantID, ChildType: postgres.NodeOA, ChildID: oaProdSecrets, ParentType: postgres.NodeOA, ParentID: oaSensitiveData, CreatedAt: now.Add(-200 * 24 * time.Hour)},

			// Subject Set -> Policy Class
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000020"), TenantID: tenantID, ChildType: postgres.NodeUA, ChildID: uaSecurity, ParentType: postgres.NodePolicyClass, ParentID: pcHighSecurity, CreatedAt: now.Add(-300 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000021"), TenantID: tenantID, ChildType: postgres.NodeUA, ChildID: uaEngineering, ParentType: postgres.NodePolicyClass, ParentID: pcDefault, CreatedAt: now.Add(-730 * 24 * time.Hour)},

			// Object Set -> Policy Class
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000022"), TenantID: tenantID, ChildType: postgres.NodeOA, ChildID: oaPublicDocs, ParentType: postgres.NodePolicyClass, ParentID: pcPublicAccess, CreatedAt: now.Add(-365 * 24 * time.Hour)},
			{ID: uuid.MustParse("60000000-0000-0000-0000-000000000023"), TenantID: tenantID, ChildType: postgres.NodeOA, ChildID: oaSensitiveData, ParentType: postgres.NodePolicyClass, ParentID: pcHighSecurity, CreatedAt: now.Add(-100 * 24 * time.Hour)},
		},
		Associations: []AssociationSeed{
			// Engineering Team can read/write Public Documents
			{UAID: uaEngineering, OAID: oaPublicDocs, Operations: []string{"read", "write"}},
			// Backend Developers can read/write/delete Private Documents
			{UAID: uaBackend, OAID: oaPrivateDocs, Operations: []string{"read", "write", "delete"}},
			// Frontend Developers can read Active Projects
			{UAID: uaFrontend, OAID: oaActiveProjects, Operations: []string{"read"}},
			// Engineering Team can read/write Repositories
			{UAID: uaEngineering, OAID: oaRepositories, Operations: []string{"read", "write"}},
			// Operations Team can read/write/delete Production Secrets
			{UAID: uaOperations, OAID: oaProdSecrets, Operations: []string{"read", "write", "delete"}},
			// Management can read/write/delete all Active Projects
			{UAID: uaManagement, OAID: oaActiveProjects, Operations: []string{"read", "write", "delete", "admin"}},
			// Security Team can read/write/delete Sensitive Data
			{UAID: uaSecurity, OAID: oaSensitiveData, Operations: []string{"read", "write", "delete", "admin"}},
			// Engineering Team can read Public APIs
			{UAID: uaEngineering, OAID: oaPublicAPIs, Operations: []string{"read"}},
			// Engineering Team can read/write Internal APIs
			{UAID: uaEngineering, OAID: oaInternalAPIs, Operations: []string{"read", "write"}},
		},
		Prohibitions: []ProhibitionSeed{
			// Prohibit individual subject (Bob) from accessing Sensitive Data
			{SubjectType: postgres.ProhibitSubject, SubjectID: subjBob, OAID: oaSensitiveData, Operations: []string{"read", "write", "delete"}},
			// Prohibit Frontend Developers from accessing Production Secrets
			{SubjectType: postgres.ProhibitUA, SubjectID: uaFrontend, OAID: oaProdSecrets, Operations: []string{"read", "write", "delete"}},
			// Prohibit individual subject (Eve) from deleting Public Documents
			{SubjectType: postgres.ProhibitSubject, SubjectID: subjEve, OAID: oaPublicDocs, Operations: []string{"delete"}},
		},
		Obligations: []postgres.Obligation{
			{
				ID:        uuid.MustParse("70000000-0000-0000-0000-000000000001"),
				TenantID:  tenantID,
				Event:     "ACCESS_GRANTED",
				Action:    datatypes.JSON(`{"type":"log","level":"info","message":"Access granted"}`),
				Condition: datatypes.JSON(`{"operation":"write","resource_type":"secret"}`),
				Enabled:   true,
				CreatedAt: now.Add(-100 * 24 * time.Hour),
			},
			{
				ID:        uuid.MustParse("70000000-0000-0000-0000-000000000002"),
				TenantID:  tenantID,
				Event:     "ACCESS_DENIED",
				Action:    datatypes.JSON(`{"type":"notify","channel":"security-alerts","message":"Access denied"}`),
				Condition: nil,
				Enabled:   true,
				CreatedAt: now.Add(-100 * 24 * time.Hour),
			},
			{
				ID:        uuid.MustParse("70000000-0000-0000-0000-000000000003"),
				TenantID:  tenantID,
				Event:     "ASSIGNMENT_ADDED",
				Action:    datatypes.JSON(`{"type":"log","level":"info","message":"Assignment added"}`),
				Condition: nil,
				Enabled:   true,
				CreatedAt: now.Add(-50 * 24 * time.Hour),
			},
		},
	}
}
