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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/config"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/internal/seed"
	"github.com/kumarabd/policy-machine/internal/server"
	"github.com/kumarabd/policy-machine/pkg/engine"
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

	// Initialize database handler with default tenant ID from engine config
	if configHandler.Postgres == nil {
		configHandler.Postgres = &postgres.Options{}
	}
	configHandler.Postgres.DefaultTenantID = configHandler.Engine.TenantID
	dbHandler, err := postgres.New(configHandler.Postgres)
	if err != nil {
		log.Error().Err(err).Msg("database initialization failed")
		os.Exit(1)
	}

	// Seed database with initial data if tenant ID is provided
	if configHandler.Engine.TenantID != "" {
		ctx := context.Background()
		if err := seed.Seed(ctx, dbHandler, configHandler.Engine.TenantID); err != nil {
			log.Warn().Err(err).Msg("failed to seed database - continuing anyway")
		} else {
			log.Info().Msg("database seeded successfully")
		}
	}

	// Initialize a new engine with the logger, metrics handler, database handler, and engine configuration
	engine := engine.New(log, metricsHandler, dbHandler, configHandler.Engine, nil, nil)
	log.Info().Msg("engine initialized")

	// Initialize both dataplane and controlplane servers
	servers, err := server.NewServers(log, metricsHandler, configHandler.Server, engine)
	if err != nil {
		log.Error().Err(err).Msg("server initialization failed")
		os.Exit(1)
	}
	log.Info().Msg("servers initialized")

	// Create a channel to control the servers
	ch := make(chan struct{})

	// Run both servers
	log.Info().Msg("servers starting")
	if err := servers.Start(ch); err != nil {
		log.Error().Err(err).Msg("failed to start servers")
		os.Exit(1)
	}
	log.Info().Msg("servers running")

	// Create a signal channel to handle graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a stop signal or engine/server completion
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
	if err := servers.Stop(); err != nil {
		log.Error().Err(err).Msg("error stopping servers")
	}
	close(ch)
}
