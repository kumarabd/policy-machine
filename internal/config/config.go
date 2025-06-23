package config

import (
	config_pkg "github.com/kumarabd/gokit/config"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/postgres"
	"github.com/kumarabd/policy-machine/pkg/server"
	"github.com/kumarabd/policy-machine/pkg/service"
)

var (
	ApplicationName    = "policy-machine"
	ApplicationVersion = "dev"
)

type Config struct {
	Server   *server.Config    `json:"server,omitempty" yaml:"server,omitempty"`
	Service  *service.Config   `json:"service" yaml:"service"`
	Metrics  *metrics.Options  `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Postgres *postgres.Options `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	//Traces  *traces.Options  `json:"traces,omitempty" yaml:"traces,omitempty"`
}

// New creates a new config instance
func New() (*Config, error) {
	// Create default config object
	configObject := &Config{
		Server:   &server.Config{},
		Service:  &service.Config{},
		Metrics:  &metrics.Options{},
		Postgres: &postgres.Options{},
	}

	finalConfig, err := config_pkg.New(configObject)
	return finalConfig.(*Config), err
}
