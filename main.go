package main

import (
	"os"

	"github.com/netresearch/ldap-manager/internal"
	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
