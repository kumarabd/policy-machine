package monitoring

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

func SetupMonitoring(cfg *Config) {
	if !cfg.Enabled {
		return
	}

	// Create metrics endpoint
	http.Handle(cfg.Path, promhttp.Handler())
	
	go func() {
		addr := ":" + strconv.Itoa(cfg.Port)
		if err := http.ListenAndServe(addr, nil); err != nil {
			// Note: In a real implementation, you'd use a proper logger here
			// logger.Error("Failed to start metrics server: %v", err)
		}
	}()
}
