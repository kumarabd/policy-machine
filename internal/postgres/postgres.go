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
		&User{},
		&Object{},
		&PolicyClass{},
		&UserAttribute{},
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

	// Composite uniqueness and helpful indexes (using raw SQL for precision)
	// You can also do these in a migration tool like goose/atlas.
	stmts := []string{
		// Users uniqueness per tenant
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_tenant_external ON users (tenant_id, external_id);`,

		// Objects uniqueness per tenant
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_objects_tenant_external ON objects (tenant_id, external_id);`,

		// UA/OA/PC unique names per tenant
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_ua_tenant_name ON user_attributes (tenant_id, name);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_oa_tenant_name ON object_attributes (tenant_id, name);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_pc_tenant_name ON policy_classes (tenant_id, name);`,

		// Assignment edge uniqueness
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_asg_edge ON assignment_edges
		 (tenant_id, child_type, child_id, parent_type, parent_id);`,

		// Association uniqueness and operations uniqueness
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_assoc_uatoa ON associations
		 (tenant_id, user_attribute_id, object_attribute_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_assocop ON association_operations
		 (tenant_id, association_id, operation);`,

		// Prohibition operation uniqueness
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_prohop ON prohibition_operations
		 (tenant_id, prohibition_id, operation);`,

		// Fast lookups for subject/object traversal
		`CREATE INDEX IF NOT EXISTS idx_asg_child_lookup ON assignment_edges (tenant_id, child_type, child_id);`,
		`CREATE INDEX IF NOT EXISTS idx_asg_parent_lookup ON assignment_edges (tenant_id, parent_type, parent_id);`,
		`CREATE INDEX IF NOT EXISTS idx_assoc_ua_lookup ON associations (tenant_id, user_attribute_id);`,
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

	return &Handler{
		H:  db,
		db: sqlDB,
	}, nil
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
