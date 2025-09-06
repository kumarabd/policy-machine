package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kumarabd/policy-machine/internal/config"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/server"
	"github.com/kumarabd/policy-machine/pkg/service"
	"github.com/kumarabd/policy-machine/pkg/store"
	"github.com/spf13/cobra"
)

// Add run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the application",
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func run() {
	// Initialize a new PostgreSQL data layer with the configuration from the config handler
	dHandler, err := store.New()
	if err != nil {
		log.Error().Err(err).Msg("unable to connect to store")
		os.Exit(1)
	}
	log.Info().Msg("store initialized")

	// Initialize Metrics instance
	metricsHandler, err := metrics.New(config.ApplicationName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Service Initialization
	service, err := service.New(log, metricsHandler, dHandler, configHandler.Service)
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	log.Info().Msg("service initialized")

	// Server Initialization
	srv, err := server.New(log, configHandler.Server, service)
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	log.Info().Msg("server listenining")

	log.Info().Msg("application starting")
	ch := make(chan struct{})
	srv.Run(ch)
	log.Info().Msg("application running")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
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
