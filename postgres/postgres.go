package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kumarabd/policy-machine/model"
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

type postgres struct {
	handler *gorm.DB
	db      *sql.DB
}

var (
	ErrNotFound = gorm.ErrRecordNotFound
)

func NewHandler(opts *Options) (*postgres, error) {
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

	// Auto migrate the schema in correct order
	// First create the base entity table
	err = db.AutoMigrate(&model.Entity{})
	if err != nil {
		return nil, err
	}

	// Then create the relationship table that references entities
	err = db.AutoMigrate(&model.Relationship{})
	if err != nil {
		return nil, err
	}

	// Then create tables that reference entities
	err = db.AutoMigrate(
		&model.Subject{},
		&model.Resource{},
		&model.Attribute{},
	)
	if err != nil {
		return nil, err
	}

	// Then create tables that reference relationships
	err = db.AutoMigrate(
		&model.Assignment{},
		&model.Association{},
		&model.Property{},
	)
	if err != nil {
		return nil, err
	}

	// First create PolicyClass table
	err = db.AutoMigrate(&model.PolicyClass{})
	if err != nil {
		return nil, err
	}

	// Then create Policy table that references PolicyClass
	err = db.AutoMigrate(&model.Policy{})
	if err != nil {
		return nil, err
	}

	return &postgres{
		handler: db,
		db:      sqlDB,
	}, nil
}

func (p *postgres) Ping() (string, error) {
	// Ping the database to ensure the connection is established
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := p.db.PingContext(ctx); err != nil {
		return "", err
	}
	return "Pong", nil
}

func (p *postgres) DB() *sql.DB {
	return p.db
}

func (h *postgres) FetchEntityForID(id string, obj *model.Entity) error {
	result := h.handler.Where("hash_id = ?", id).First(obj)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchRelationshipsForSource(id string, relationships *[]model.Relationship) error {
	result := h.handler.Where("from_id = ?", id).Find(relationships)
	if result.Error != nil {
		return result.Error
	}
	return nil
}
