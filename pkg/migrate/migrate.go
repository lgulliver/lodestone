package migrate

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lgulliver/lodestone/pkg/config"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/rs/zerolog/log"
)

// Migrator handles database migrations
type Migrator struct {
	db            *sql.DB
	migrationsFS  fs.FS
	migrationsDir string
}

// NewMigrator creates a new migration runner
func NewMigrator(cfg *config.DatabaseConfig, migrationsFS fs.FS, migrationsDir string) (*Migrator, error) {
	dsn := cfg.DatabaseURL()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Migrator{
		db:            db,
		migrationsFS:  migrationsFS,
		migrationsDir: migrationsDir,
	}, nil
}

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// EnsureMigrationsTable creates the migrations tracking table if it doesn't exist
func (m *Migrator) EnsureMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`

	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// GetAppliedMigrations returns a list of applied migration versions
func (m *Migrator) GetAppliedMigrations() ([]int, error) {
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// LoadMigrations loads all migration files from the embedded filesystem
func (m *Migrator) LoadMigrations() ([]*Migration, error) {
	entries, err := fs.ReadDir(m.migrationsFS, m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []*Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		migration, err := m.parseMigrationFile(entry.Name())
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("Skipping invalid migration file")
			continue
		}

		migrations = append(migrations, migration)
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parseMigrationFile parses a migration file and extracts up/down SQL
func (m *Migrator) parseMigrationFile(filename string) (*Migration, error) {
	// Parse version from filename (e.g., "001_initial_schema.sql")
	parts := strings.Split(filename, "_")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid migration filename format: %s", filename)
	}

	var version int
	if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
		return nil, fmt.Errorf("failed to parse version from filename %s: %w", filename, err)
	}

	name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".sql")

	// Read file content
	filePath := filepath.Join(m.migrationsDir, filename)
	content, err := fs.ReadFile(m.migrationsFS, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file %s: %w", filename, err)
	}

	// Split into up and down parts
	contentStr := string(content)
	upSQL, downSQL := m.splitMigration(contentStr)

	return &Migration{
		Version: version,
		Name:    name,
		UpSQL:   upSQL,
		DownSQL: downSQL,
	}, nil
}

// splitMigration splits migration content into up and down parts
func (m *Migrator) splitMigration(content string) (string, string) {
	lines := strings.Split(content, "\n")
	var upLines, downLines []string
	var inDown bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "-- +migrate Up" {
			inDown = false
			continue
		}
		if trimmed == "-- +migrate Down" {
			inDown = true
			continue
		}

		if inDown {
			downLines = append(downLines, line)
		} else {
			upLines = append(upLines, line)
		}
	}

	return strings.Join(upLines, "\n"), strings.Join(downLines, "\n")
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	if err := m.EnsureMigrationsTable(); err != nil {
		return err
	}

	appliedVersions, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	appliedMap := make(map[int]bool)
	for _, version := range appliedVersions {
		appliedMap[version] = true
	}

	var pendingMigrations []*Migration
	for _, migration := range migrations {
		if !appliedMap[migration.Version] {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	if len(pendingMigrations) == 0 {
		log.Info().Msg("No pending migrations")
		return nil
	}

	log.Info().Int("count", len(pendingMigrations)).Msg("Running pending migrations")

	for _, migration := range pendingMigrations {
		if err := m.runMigrationUp(migration); err != nil {
			return fmt.Errorf("failed to run migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		log.Info().Int("version", migration.Version).Str("name", migration.Name).Msg("Applied migration")
	}

	return nil
}

// Down rolls back the last migration
func (m *Migrator) Down() error {
	if err := m.EnsureMigrationsTable(); err != nil {
		return err
	}

	appliedVersions, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	if len(appliedVersions) == 0 {
		log.Info().Msg("No migrations to roll back")
		return nil
	}

	// Get the last applied migration
	lastVersion := appliedVersions[len(appliedVersions)-1]

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	var targetMigration *Migration
	for _, migration := range migrations {
		if migration.Version == lastVersion {
			targetMigration = migration
			break
		}
	}

	if targetMigration == nil {
		return fmt.Errorf("migration file for version %d not found", lastVersion)
	}

	if err := m.runMigrationDown(targetMigration); err != nil {
		return fmt.Errorf("failed to roll back migration %d (%s): %w", targetMigration.Version, targetMigration.Name, err)
	}

	log.Info().Int("version", targetMigration.Version).Str("name", targetMigration.Name).Msg("Rolled back migration")
	return nil
}

// runMigrationUp executes the up part of a migration
func (m *Migrator) runMigrationUp(migration *Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the migration
	if _, err := tx.Exec(migration.UpSQL); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record the migration
	if _, err := tx.Exec("INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", migration.Version, migration.Name); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// runMigrationDown executes the down part of a migration
func (m *Migrator) runMigrationDown(migration *Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the rollback
	if _, err := tx.Exec(migration.DownSQL); err != nil {
		return fmt.Errorf("failed to execute rollback SQL: %w", err)
	}

	// Remove the migration record
	if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", migration.Version); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return tx.Commit()
}

// Close closes the database connection
func (m *Migrator) Close() error {
	return m.db.Close()
}
