// Package main Policy Machine + Auth Engine API
//
// A unified access control system that provides a simple authorization API
// alongside comprehensive policy management capabilities.
//
// ## Quick Start
//
// **Main Authorization Endpoint:** POST /api/v1/authorize
// - Works with all access control models (RBAC, ABAC, ReBAC)
// - Single endpoint for all authorization decisions
//
// **Policy Management:** /api/v1/policies/*
// - Universal policy CRUD operations
// - Policy validation and versioning
//
// ## Advanced APIs
//
// For users who need specific access control models:
// - **RBAC (Role-Based):** /api/v1/rbac/*
// - **ABAC (Attribute-Based):** /api/v1/abac/*
// - **ReBAC (Relationship-Based):** /api/v1/rebac/*
//
// ## Internal APIs
//
// Expert-only APIs for advanced use cases:
// - **NGAC (Next Generation AC):** /api/v1/ngac/*
//
//	Schemes: http, https
//	BasePath: /
//	Version: 1.0.0
//	Host: localhost:8080
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	SecurityDefinitions:
//	Bearer:
//	  type: apiKey
//	  name: Authorization
//	  in: header
//
// swagger:meta
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/config"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/postgres"
	"github.com/kumarabd/policy-machine/pkg/server"
	"github.com/kumarabd/policy-machine/pkg/service"
)

// main is the entry point of the application
func main() {
	// Initialize a new logger with the application name and syslog format
	log, err := logger.New(config.ApplicationName, logger.Options{
		Format: logger.SyslogLogFormat,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize a new configuration handler
	configHandler, err := config.New()
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}

	// Initialize a new metrics handler with the application name and server namespace
	metricsHandler, err := metrics.New(config.ApplicationName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize in-memory cache layer
	dbHandler, err := postgres.New(configHandler.Postgres)
	if err != nil {
		log.Error().Err(err).Msg("cache initialization failed")
		os.Exit(1)
	}

	// Initialize a new service with the logger, metrics handler, data layer, and service configuration
	service, err := service.New(log, metricsHandler, configHandler.Service)
	if err != nil {
		log.Error().Err(err).Msg("service initialization failed")
		os.Exit(1)
	}
	service.InitStore(dbHandler)
	log.Info().Msg("service initialized")

	// Initialize a new server with the logger, metrics handler, server configuration, and service
	srv, err := server.New(config.ApplicationName, log, metricsHandler, configHandler.Server, service)
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	log.Info().Msg("server initialized")

	// Create a channel to control the server
	ch := make(chan struct{})

	// Run the server
	log.Info().Msg("server starting")
	srv.Start(ch)
	log.Info().Msg("server running")

	// Create a signal channel to handle graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a stop signal or service/server completion
	exit := false
	for !exit {
		select {
		case <-ch:
			exit = true
		case <-signalChan:
			exit = true
		}
	}

	log.Info().Msg("received stop. gracefully shutting down...")
	close(ch)
}
