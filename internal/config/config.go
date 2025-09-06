package config

import (
	config_pkg "github.com/kumarabd/gokit/config"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/server"
	"github.com/kumarabd/policy-machine/pkg/service"
	"github.com/spf13/cobra"
)

var (
	ApplicationName    = "default"
	ApplicationVersion = "dev"
)

type Config struct {
	Server  *server.Config   `json:"server,omitempty" yaml:"server,omitempty"`
	Service *service.Config  `json:"service" yaml:"service"`
	Metrics *metrics.Options `json:"metrics,omitempty" yaml:"metrics,omitempty"`
}

// New creates a new config instance
func New(cmd *cobra.Command) (*Config, error) {
	// Create default config object
	configObject := &Config{
		Server:  &server.Config{},
		Service: &service.Config{},
		Metrics: &metrics.Options{},
	}

	finalConfig, err := config_pkg.NewWithCommand(cmd, configObject)
	return finalConfig.(*Config), err
}
