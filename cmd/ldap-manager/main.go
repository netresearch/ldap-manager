// Package main provides the entry point for the LDAP Manager web application.
// It initializes logging, parses configuration options, and starts the web server.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/version"
	"github.com/netresearch/ldap-manager/internal/web"
)

const shutdownTimeout = 30 * time.Second

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msgf("LDAP Manager %s starting...", version.FormatVersion())

	opts, err := options.Parse()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse configuration")
	}
	log.Logger = log.Logger.Level(opts.LogLevel)

	app, err := web.NewApp(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize web app")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := app.Listen(ctx, ":3000"); err != nil {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case err := <-serverErr:
		log.Error().Err(err).Msg("Server error")
	}

	// Initiate graceful shutdown
	log.Info().Msg("Initiating graceful shutdown...")
	cancel() // Signal all goroutines to stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		shutdownCancel()
		os.Exit(1) //nolint:gocritic // Exit is intentional after shutdown error
	}

	log.Info().Msg("Graceful shutdown complete")
}
