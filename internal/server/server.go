package server

import (
	"strings"

	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/engine"
	"github.com/kumarabd/policy-machine/internal/http"
)

// Server defines the interface for any server implementation (HTTP, gRPC, etc.)
type Server interface {
	Start(ch chan struct{}) error
	Stop() error
}

// BaseServerConfig holds base server configuration (port, etc.)
type BaseServerConfig struct {
	Port int64 `json:"port" yaml:"port"`
}

// Config holds server configuration
type Config struct {
	Name string            `json:"name" yaml:"name"`
	Base *BaseServerConfig `json:"base" yaml:"base"`
}

// New creates a new server instance based on configuration
// Currently returns HTTP server, but can be extended to support other protocols
func New(l *logger.Handler, m *metrics.Handler, config *Config, eng *engine.Engine) (Server, error) {
	// For now, always return HTTP server
	// In the future, this could check config to return HTTP, gRPC, or other implementations
	return newHTTPServer(l, m, config, eng)
}

// newHTTPServer creates an HTTP server instance
// This is a private function - external code should use New() which returns the Server interface
func newHTTPServer(l *logger.Handler, m *metrics.Handler, config *Config, eng *engine.Engine) (Server, error) {
	// Convert server.Config to http.Config
	httpConfig := &http.Config{
		Port: 8500, // Default port
	}
	if config.Base != nil {
		httpConfig.Port = config.Base.Port
	}

	// Directly create HTTP server using dependency injection
	return http.New(l, m, httpConfig, eng)
}

// formatTitle converts "abc-def" format to "Abc Def" format
// Examples: "policy-machine" -> "Policy Machine", "auth-service" -> "Auth Service"
func formatTitle(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}
