package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	postgres_pkg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Endpoint struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Host     string `json:"host" yaml:"host"`
	Port     string `json:"port" yaml:"port"`
	DB       string `json:"db" yaml:"db"`
	SSLMode  string `json:"ssl_mode" yaml:"ssl_mode"`
}

type Options struct {
	// Endpoints is a list of URLs to connect to.
	Endpoint *Endpoint `json:"endpoint" yaml:"endpoint"`
	// DialTimeoutSeconds sets the timeout for dialing to an endpoint.
	DialTimeoutSeconds int `json:"dial_timeout_seconds,string" yaml:"dial_timeout_seconds"`
	// MaxRetries is the maximum number of retries before giving up on a request.
	MaxRetries int `json:"max_retries,string" yaml:"max_retries"`
	// DefaultTenantID is the default tenant ID to seed into the database
	DefaultTenantID string `json:"default_tenant_id,omitempty" yaml:"default_tenant_id,omitempty"`
}

type Handler struct {
	H  *gorm.DB
	db *sql.DB
}

var (
	ErrNotFound = gorm.ErrRecordNotFound
)

func New(opts *Options) (*Handler, error) {
	// Set up GORM with PostgreSQL
	config := postgres_pkg.Config{
		DSN: fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=America/Los_Angeles", opts.Endpoint.Host, opts.Endpoint.Username, opts.Endpoint.Password, opts.Endpoint.DB, opts.Endpoint.Port, opts.Endpoint.SSLMode),
		// DSN:                  fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", opts.Endpoint.Username, opts.Endpoint.Password, opts.Endpoint.Host, opts.Endpoint.Port, opts.Endpoint.DB),
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second,   // Slow SQL threshold
			LogLevel:      logger.Silent, // Log level
			Colorful:      false,         // Disable color
		},
	)

	db, err := gorm.Open(postgres_pkg.New(config), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetConnMaxIdleTime(time.Duration(opts.DialTimeoutSeconds) * time.Second)
	sqlDB.SetConnMaxLifetime(time.Duration(opts.DialTimeoutSeconds) * time.Second)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// Auto migrate the schema
	err = db.AutoMigrate(
		&Tenant{},
		&Subject{},
		&Object{},
		&PolicyClass{},
		&SubjectAttribute{},
		&ObjectAttribute{},
		&AssignmentEdge{},
		&Association{},
		&AssociationOperation{},
		&Prohibition{},
		&ProhibitionOperation{},
		&Obligation{},
		&PolicyRevision{},
		&PolicyChange{},
	)
	if err != nil {
		return nil, err
	}

	// Drop any incorrectly created unique index that GORM might have created
	// This ensures we use our SQL-defined index instead
	// GORM might create indexes with different naming conventions
	// We need to drop all possible variations
	dropIndexStmts := []string{
		`DROP INDEX IF EXISTS assignment_edges_uidx_asg_edge;`,
		`DROP INDEX IF EXISTS uidx_asg_edge;`,
		`DROP INDEX IF EXISTS assignment_edges_uidx_asg_edge_1;`,
		`DROP INDEX IF EXISTS uidx_asg_edge_1;`,
	}
	for _, stmt := range dropIndexStmts {
		if err := db.Exec(stmt).Error; err != nil {
			// Ignore errors when dropping indexes that don't exist
			log.Printf("Warning: failed to drop index (may not exist): %v", err)
		}
	}

	// Composite uniqueness and helpful indexes (using raw SQL for precision)
	// You can also do these in a migration tool like goose/atlas.
	stmts := []string{
		// Subjects uniqueness per tenant
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_subjects_tenant_external ON subjects (tenant_id, external_id);`,

		// Objects uniqueness per tenant
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_objects_tenant_external ON objects (tenant_id, external_id);`,
		`CREATE INDEX IF NOT EXISTS idx_objects_tenant_attribute_type ON objects (tenant_id, attribute_type);`,

		// UA/OA/PC unique names per tenant
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_ua_tenant_name ON subject_attributes (tenant_id, name);`,
		`CREATE INDEX IF NOT EXISTS idx_ua_type ON subject_attributes (tenant_id, attribute_type);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_oa_tenant_name ON object_attributes (tenant_id, name);`,
		`CREATE INDEX IF NOT EXISTS idx_oa_type ON object_attributes (tenant_id, attribute_type);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_pc_tenant_name ON policy_classes (tenant_id, name);`,

		// Assignment edge uniqueness
		// Note: We use CREATE UNIQUE INDEX (not IF NOT EXISTS) after dropping to ensure clean state
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_asg_edge ON assignment_edges (tenant_id, child_type, child_id, parent_type, parent_id);`,

		// Association uniqueness and operations uniqueness
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_assoc_uatoa ON associations
		 (tenant_id, subject_attribute_id, object_attribute_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_assocop ON association_operations
		 (tenant_id, association_id, operation);`,

		// Prohibition operation uniqueness
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_prohop ON prohibition_operations
		 (tenant_id, prohibition_id, operation);`,

		// Fast lookups for subject/object traversal
		`CREATE INDEX IF NOT EXISTS idx_asg_child_lookup ON assignment_edges (tenant_id, child_type, child_id);`,
		`CREATE INDEX IF NOT EXISTS idx_asg_parent_lookup ON assignment_edges (tenant_id, parent_type, parent_id);`,
		`CREATE INDEX IF NOT EXISTS idx_assoc_ua_lookup ON associations (tenant_id, subject_attribute_id);`,
		`CREATE INDEX IF NOT EXISTS idx_assoc_oa_lookup ON associations (tenant_id, object_attribute_id);`,

		// Fast lookups for policy changes
		`CREATE INDEX IF NOT EXISTS idx_changes_tenant_seq ON policy_changes (tenant_id, seq);`,
		`CREATE INDEX IF NOT EXISTS idx_changes_tenant_rev ON policy_changes (tenant_id, revision);`,
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return nil, err
		}
	}

	// Seed default tenant from config if provided
	if opts.DefaultTenantID != "" {
		if err := seedDefaultTenant(db, opts.DefaultTenantID); err != nil {
			return nil, fmt.Errorf("failed to seed default tenant: %w", err)
		}
	}

	handler := &Handler{
		H:  db,
		db: sqlDB,
	}

	// Seed data if default tenant is provided
	// Note: We defer seed data loading to avoid circular dependencies
	// The seed package will be called from main.go after initialization
	if opts.DefaultTenantID != "" {
		// Seed data will be loaded separately to avoid import cycles
		// See cmd/main.go for seed initialization
	}

	return handler, nil
}

// seedDefaultTenant seeds the default tenant into the database if it doesn't exist
func seedDefaultTenant(db *gorm.DB, tenantID string) error {
	// Check if tenant already exists
	var existing Tenant
	result := db.Where("id = ?", tenantID).First(&existing)
	if result.Error == nil {
		// Tenant already exists, skip
		return nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		// Some other error occurred
		return result.Error
	}

	// Create the default tenant
	tenant := Tenant{
		ID:   tenantID,
		Name: tenantID, // Use tenant ID as name
	}
	if err := db.Create(&tenant).Error; err != nil {
		return err
	}

	return nil
}

func (p *Handler) Ping() (bool, error) {
	// Ping the database to ensure the connection is established
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := p.db.PingContext(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (p *Handler) DB() *sql.DB {
	return p.db
}
