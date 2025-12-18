package server

import (
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/engine"
	"github.com/kumarabd/policy-machine/internal/http/dataplane"
	"github.com/kumarabd/policy-machine/internal/http/controlplane"
)

// Server defines the interface for any server implementation (HTTP, gRPC, etc.)
type Server interface {
	Start(ch chan struct{}) error
	Stop() error
}

// DataplaneConfig holds dataplane server configuration
type DataplaneConfig struct {
	Port int64 `json:"port" yaml:"port"`
}

// ControlplaneConfig holds controlplane server configuration
type ControlplaneConfig struct {
	Port int64 `json:"port" yaml:"port"`
}

// Config holds server configuration for both dataplane and controlplane
type Config struct {
	Name         string             `json:"name" yaml:"name"`
	Dataplane    *DataplaneConfig  `json:"dataplane" yaml:"dataplane"`
	Controlplane *ControlplaneConfig `json:"controlplane" yaml:"controlplane"`
}

// Servers holds both dataplane and controlplane server instances
type Servers struct {
	Dataplane    Server
	Controlplane Server
}

// NewServers creates both dataplane and controlplane server instances
func NewServers(l *logger.Handler, m *metrics.Handler, config *Config, eng *engine.Engine) (*Servers, error) {
	// Create dataplane server
	dataplaneConfig := &dataplane.Config{
		Port: 8500, // Default port
	}
	if config.Dataplane != nil && config.Dataplane.Port > 0 {
		dataplaneConfig.Port = config.Dataplane.Port
	}

	dataplaneSrv, err := dataplane.New(l, m, dataplaneConfig, eng)
	if err != nil {
		return nil, err
	}

	// Create controlplane server
	controlplaneConfig := &controlplane.Config{
		Port: 8501, // Default port
	}
	if config.Controlplane != nil && config.Controlplane.Port > 0 {
		controlplaneConfig.Port = config.Controlplane.Port
	}

	controlplaneSrv, err := controlplane.New(l, m, controlplaneConfig, eng)
	if err != nil {
		return nil, err
	}

	return &Servers{
		Dataplane:    dataplaneSrv,
		Controlplane: controlplaneSrv,
	}, nil
}

// Start starts both servers
func (s *Servers) Start(ch chan struct{}) error {
	// Start dataplane server
	if err := s.Dataplane.Start(ch); err != nil {
		return err
	}

	// Start controlplane server
	if err := s.Controlplane.Start(ch); err != nil {
		return err
	}

	return nil
}

// Stop stops both servers
func (s *Servers) Stop() error {
	if err := s.Dataplane.Stop(); err != nil {
		return err
	}
	if err := s.Controlplane.Stop(); err != nil {
		return err
	}
	return nil
}
