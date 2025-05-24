package common

import (
	"fmt"

	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database wraps the GORM database connection
type Database struct {
	*gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	dsn := cfg.DatabaseURL()
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &Database{DB: db}, nil
}

// Migrate runs database migrations
func (db *Database) Migrate() error {
	return db.AutoMigrate(
		&types.User{},
		&types.APIKey{},
		&types.Artifact{},
		&types.Permission{},
	)
}

// Close closes the database connection
func (db *Database) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
