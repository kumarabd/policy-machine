package main

import (
	"fmt"
	"os"

	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/config"
	"github.com/spf13/cobra"
)

var (
	log           *logger.Handler
	configHandler *config.Config
	cmd           *cobra.Command
)

func main() {
	// Initialize Logger instance
	var err error
	log, err = logger.New(config.ApplicationName, logger.Options{
		Format: logger.SyslogLogFormat,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create a root command for handling flags
	cmd = &cobra.Command{
		Use:   config.ApplicationName,
		Short: config.ApplicationName,
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	// Config init and seed
	configHandler, err = config.New(cmd)
	if err != nil {
		log.Error().Err(err).Msg("unable to initialize config")
		os.Exit(1)
	}

	// Add commands to root
	cmd.AddCommand(runCmd)

	// Execute the root command
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
