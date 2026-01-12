// Package main provides the entry point for the LDAP Manager web application.
// It initializes logging, parses configuration options, and starts the web server.
package main

import (
	"context"
	"net/http"
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

const (
	shutdownTimeout     = 30 * time.Second
	healthCheckTimeout  = 3 * time.Second
	healthCheckEndpoint = "http://localhost:3000/health/live"
)

func main() {
	// Handle --health-check flag early, before any other initialization
	if len(os.Args) == 2 && os.Args[1] == "--health-check" {
		os.Exit(runHealthCheck())
	}

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
		shutdownCancel() // Required: os.Exit does not run deferred functions
		os.Exit(1)       //nolint:gocritic // Exit is intentional after shutdown error
	}

	log.Info().Msg("Graceful shutdown complete")
}

// runHealthCheck performs an HTTP health check against the running application.
// Returns 0 if healthy (HTTP 200), 1 otherwise.
// Used by Docker HEALTHCHECK to verify the application is running correctly.
func runHealthCheck() int {
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthCheckEndpoint, nil)
	if err != nil {
		return 1
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return 0
	}

	return 1
}
