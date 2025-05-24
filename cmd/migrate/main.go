package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/migrate"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	var (
		up   = flag.Bool("up", false, "Run pending migrations")
		down = flag.Bool("down", false, "Roll back the last migration")
	)
	flag.Parse()

	if !*up && !*down {
		fmt.Printf("Usage: %s [-up | -down]\n", os.Args[0])
		fmt.Println("  -up    Run pending migrations")
		fmt.Println("  -down  Roll back the last migration")
		os.Exit(1)
	}

	// Load configuration
	cfg := config.LoadFromEnv()
	cfg.Logging.SetupLogging()

	// Create migrator
	migrator, err := migrate.NewMigrator(&cfg.Database, migrationsFS, "migrations")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create migrator")
	}
	defer migrator.Close()

	// Run migrations
	if *up {
		if err := migrator.Up(); err != nil {
			log.Fatal().Err(err).Msg("Failed to run migrations")
		}
		log.Info().Msg("Migrations completed successfully")
	}

	if *down {
		if err := migrator.Down(); err != nil {
			log.Fatal().Err(err).Msg("Failed to roll back migration")
		}
		log.Info().Msg("Rollback completed successfully")
	}
}
