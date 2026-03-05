package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kumarabd/policy-machine/internal/postgres"
	"gorm.io/gorm"
)

// Seed populates the database with seed data
// It is idempotent and can be safely called multiple times
func Seed(ctx context.Context, db *postgres.Handler, tenantID string) error {
	data := GetSeedData(tenantID)

	// Create tenant if it doesn't exist
	if err := ensureTenant(ctx, db, tenantID); err != nil {
		return fmt.Errorf("failed to ensure tenant: %w", err)
	}

	// Seed subjects
	for _, subject := range data.Subjects {
		if _, err := db.CreateSubject(ctx, tenantID, &subject); err != nil {
			return fmt.Errorf("failed to create subject %s: %w", subject.ExternalID, err)
		}
	}

	// Seed subject sets (SubjectAttributes)
	for _, ua := range data.SubjectSets {
		if _, err := db.CreateSubjectSet(ctx, tenantID, &ua); err != nil {
			return fmt.Errorf("failed to create subject set %s: %w", ua.Name, err)
		}
	}

	// Seed objects
	for _, obj := range data.Objects {
		if _, err := db.CreateObject(ctx, tenantID, &obj); err != nil {
			return fmt.Errorf("failed to create object %s: %w", obj.ExternalID, err)
		}
	}

	// Seed object sets (ObjectAttributes)
	for _, oa := range data.ObjectSets {
		if _, err := db.CreateObjectSet(ctx, tenantID, &oa); err != nil {
			return fmt.Errorf("failed to create object set %s: %w", oa.Name, err)
		}
	}

	// Seed policy classes
	for _, pc := range data.PolicyClasses {
		if err := createPolicyClass(ctx, db, tenantID, &pc); err != nil {
			return fmt.Errorf("failed to create policy class %s: %w", pc.Name, err)
		}
	}

	// Seed relationships (AssignmentEdges)
	// Use raw database connection to completely bypass GORM's index handling
	sqlDB := db.DB()
	for _, edge := range data.Relationships {
		// Check if edge already exists using raw SQL
		var count int
		checkSQL := `SELECT COUNT(*) FROM assignment_edges 
			WHERE tenant_id = $1 AND child_type = $2 AND child_id = $3 AND parent_type = $4 AND parent_id = $5`
		err := sqlDB.QueryRowContext(ctx, checkSQL,
			tenantID, string(edge.ChildType), edge.ChildID, string(edge.ParentType), edge.ParentID).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to check existing relationship: %w", err)
		}
		if count > 0 {
			// Already exists - skip (idempotent)
			continue
		}

		// Insert using raw SQL with PostgreSQL native placeholders
		// This completely bypasses GORM and any index handling
		// Use ON CONFLICT with column list (matches our unique index)
		insertSQL := `INSERT INTO assignment_edges (id, tenant_id, child_type, child_id, parent_type, parent_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (tenant_id, child_type, child_id, parent_type, parent_id) DO NOTHING`
		_, err = sqlDB.ExecContext(ctx, insertSQL,
			edge.ID, tenantID, string(edge.ChildType), edge.ChildID, string(edge.ParentType), edge.ParentID, edge.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert relationship (edge: %s -> %s): %w", edge.ChildID, edge.ParentID, err)
		}
	}

	// Seed associations (rules)
	// Note: "subject-set" and "object-set" are API terminology only for the subject-sets/object-sets endpoints.
	// In rules API, we use "subject-attribute" and "object-attribute" to refer to the actual entities.
	for _, assoc := range data.Associations {
		// Seed data uses UA->OA associations (subject-attribute -> object-attribute)
		if _, _, err := db.CreateRule(ctx, tenantID, "subject-attribute", assoc.UAID, "object-attribute", assoc.OAID, assoc.Operations); err != nil {
			return fmt.Errorf("failed to create rule UA=%s OA=%s: %w", assoc.UAID, assoc.OAID, err)
		}
	}

	// Seed prohibitions (denies)
	for _, proh := range data.Prohibitions {
		if _, _, err := db.CreateDeny(ctx, tenantID, proh.SubjectType, proh.SubjectID, proh.OAID, proh.Operations); err != nil {
			return fmt.Errorf("failed to create prohibition: %w", err)
		}
	}

	// Seed obligations
	for _, obl := range data.Obligations {
		if err := createObligation(ctx, db, tenantID, &obl); err != nil {
			return fmt.Errorf("failed to create obligation: %w", err)
		}
	}

	return nil
}

// ensureTenant ensures the tenant exists
func ensureTenant(ctx context.Context, db *postgres.Handler, tenantID string) error {
	var existing postgres.Tenant
	result := db.H.WithContext(ctx).Where("id = ?", tenantID).First(&existing)
	if result.Error == nil {
		// Tenant already exists
		return nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}

	// Create the tenant
	tenant := postgres.Tenant{
		ID:   tenantID,
		Name: tenantID,
	}
	if err := db.H.WithContext(ctx).Create(&tenant).Error; err != nil {
		return err
	}

	return nil
}

// createPolicyClass creates a policy class if it doesn't exist
func createPolicyClass(ctx context.Context, db *postgres.Handler, tenantID string, pc *postgres.PolicyClass) error {
	pc.TenantID = tenantID

	// Check if policy class already exists (idempotent)
	var existing postgres.PolicyClass
	err := db.H.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantID, pc.Name).First(&existing).Error
	if err == nil {
		// Policy class already exists, skip
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Policy class doesn't exist, create it
	return db.H.WithContext(ctx).Create(pc).Error
}

// createObligation creates an obligation if it doesn't exist
func createObligation(ctx context.Context, db *postgres.Handler, tenantID string, obl *postgres.Obligation) error {
	obl.TenantID = tenantID

	// Check if obligation already exists (idempotent)
	var existing postgres.Obligation
	err := db.H.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, obl.ID).First(&existing).Error
	if err == nil {
		// Obligation already exists, skip
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Obligation doesn't exist, create it
	return db.H.WithContext(ctx).Create(obl).Error
}
