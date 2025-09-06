// Package main provides the entry point for the LDAP Manager web application.
// It initializes logging, parses configuration options, and starts the web server.
package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal"
	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msgf("LDAP Manager %s starting...", internal.FormatVersion())

	opts := options.Parse()
	log.Logger = log.Logger.Level(opts.LogLevel)

	app, err := web.NewApp(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize web app")
	}

	if err := app.Listen(":3000"); err != nil {
		log.Fatal().Err(err).Msg("could not start web server")
	}
}
