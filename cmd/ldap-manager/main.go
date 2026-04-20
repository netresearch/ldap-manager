// Package main provides the entry point for the LDAP Manager web application.
// It initializes logging, parses configuration options, and starts the web server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	shutdownTimeout    = 30 * time.Second
	healthCheckTimeout = 3 * time.Second
	defaultPort        = "3000"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Handle version and health-check flags early, before any other initialization
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "version", "--version":
			if len(os.Args) == 3 && os.Args[2] == "--json" {
				info := map[string]string{
					"version":   version.Version,
					"commit":    version.CommitHash,
					"buildTime": version.BuildTimestamp,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(info)
				os.Exit(0)
			}
			fmt.Println(version.FormatVersion())
			os.Exit(0)
		case "--health-check":
			os.Exit(runHealthCheck(port))
		}
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
		if err := app.Listen(ctx, ":"+port); err != nil {
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
//
// The target URL is constructed from the PORT environment variable for the
// localhost health endpoint only; the scheme and host are hardcoded and the
// port is validated below, so the URL cannot point at arbitrary external
// hosts. The gosec G704 SSRF warnings are therefore suppressed with inline
// annotations.
func runHealthCheck(port string) int {
	// Validate port is purely numeric and in range before building the URL.
	// This prevents anything surprising (like embedded slashes or auth info)
	// from reaching http.NewRequestWithContext.
	if !isValidPort(port) {
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	url := "http://localhost:" + port + "/health/live"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // #nosec G704 -- URL is localhost with a validated numeric port
	if err != nil {
		return 1
	}

	client := &http.Client{}
	resp, err := client.Do(req) // #nosec G704 -- request URL is localhost with a validated numeric port (see comment above)
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return 0
	}

	return 1
}

// isValidPort reports whether s is a non-empty decimal port number in 1..65535.
func isValidPort(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}

	return n >= 1 && n <= 65535
}
