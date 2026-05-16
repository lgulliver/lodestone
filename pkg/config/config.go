package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds the configuration for all services
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// StorageConfig holds blob storage configuration
type StorageConfig struct {
	Type      string             `yaml:"type"`       // s3, gcs, azure, local
	Bucket    string             `yaml:"bucket"`     // legacy alias for S3 bucket
	Region    string             `yaml:"region"`     // legacy alias for S3 region
	Endpoint  string             `yaml:"endpoint"`   // legacy alias for S3 endpoint
	AccessKey string             `yaml:"access_key"` // legacy alias for S3 access key
	SecretKey string             `yaml:"secret_key"` // legacy alias for S3 secret key
	LocalPath string             `yaml:"local_path"`
	S3        S3StorageConfig    `yaml:"s3"`
	Azure     AzureStorageConfig `yaml:"azure"`
	Options   map[string]string  `yaml:"options"`
}

// S3StorageConfig holds S3-specific storage settings.
type S3StorageConfig struct {
	Bucket         string `yaml:"bucket"`
	Region         string `yaml:"region"`
	Endpoint       string `yaml:"endpoint"`
	AccessKey      string `yaml:"access_key"`
	SecretKey      string `yaml:"secret_key"`
	ForcePathStyle bool   `yaml:"force_path_style"`
}

// AzureStorageConfig holds Azure Blob-specific storage settings.
type AzureStorageConfig struct {
	AccountName      string `yaml:"account_name"`
	AccountKey       string `yaml:"account_key"`
	Container        string `yaml:"container"`
	Endpoint         string `yaml:"endpoint"`
	ConnectionString string `yaml:"connection_string"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	JWTSecret     string        `yaml:"jwt_secret"`
	JWTExpiration time.Duration `yaml:"jwt_expiration"`
	BCryptCost    int           `yaml:"bcrypt_cost"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json, text
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "lodestone"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "lodestone"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Storage: StorageConfig{
			Type:      getEnv("STORAGE_TYPE", "local"),
			LocalPath: getEnv("STORAGE_LOCAL_PATH", "./artifacts"),
			Bucket:    getStorageEnv("STORAGE_S3_BUCKET", "S3_BUCKET", "STORAGE_BUCKET", "lodestone-artifacts"),
			Region:    getStorageEnv("STORAGE_S3_REGION", "S3_REGION", "STORAGE_REGION", "us-east-1"),
			Endpoint:  getStorageEnv("STORAGE_S3_ENDPOINT", "S3_ENDPOINT", "STORAGE_ENDPOINT", ""),
			AccessKey: getStorageEnv("STORAGE_S3_ACCESS_KEY", "S3_ACCESS_KEY", "STORAGE_ACCESS_KEY", ""),
			SecretKey: getStorageEnv("STORAGE_S3_SECRET_KEY", "S3_SECRET_KEY", "STORAGE_SECRET_KEY", ""),
			S3: S3StorageConfig{
				Bucket:         getStorageEnv("STORAGE_S3_BUCKET", "S3_BUCKET", "STORAGE_BUCKET", "lodestone-artifacts"),
				Region:         getStorageEnv("STORAGE_S3_REGION", "S3_REGION", "STORAGE_REGION", "us-east-1"),
				Endpoint:       getStorageEnv("STORAGE_S3_ENDPOINT", "S3_ENDPOINT", "STORAGE_ENDPOINT", ""),
				AccessKey:      getStorageEnv("STORAGE_S3_ACCESS_KEY", "S3_ACCESS_KEY", "STORAGE_ACCESS_KEY", ""),
				SecretKey:      getStorageEnv("STORAGE_S3_SECRET_KEY", "S3_SECRET_KEY", "STORAGE_SECRET_KEY", ""),
				ForcePathStyle: getEnvBool("STORAGE_S3_FORCE_PATH_STYLE", false),
			},
			Azure: AzureStorageConfig{
				AccountName:      getStorageEnv("STORAGE_AZURE_ACCOUNT_NAME", "AZURE_STORAGE_ACCOUNT", "", ""),
				AccountKey:       getStorageEnv("STORAGE_AZURE_ACCOUNT_KEY", "AZURE_STORAGE_KEY", "", ""),
				Container:        getStorageEnv("STORAGE_AZURE_CONTAINER", "AZURE_STORAGE_CONTAINER", "", "lodestone-artifacts"),
				Endpoint:         getStorageEnv("STORAGE_AZURE_ENDPOINT", "AZURE_STORAGE_ENDPOINT", "", ""),
				ConnectionString: getStorageEnv("STORAGE_AZURE_CONNECTION_STRING", "AZURE_STORAGE_CONNECTION_STRING", "", ""),
			},
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", "your-secret-key"),
			JWTExpiration: getEnvDuration("JWT_EXPIRATION", 24*time.Hour),
			BCryptCost:    getEnvInt("BCRYPT_COST", 12),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}
}

// DatabaseURL returns a PostgreSQL connection string
func (d *DatabaseConfig) DatabaseURL() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// RedisAddr returns the Redis address
func (r *RedisConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// SetupLogging configures zerolog based on the LoggingConfig
func (c *LoggingConfig) SetupLogging() {
	// Set log level
	switch c.Level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Set log format
	if c.Format == "text" || c.Format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	// Default is JSON format, which is already set by zerolog
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getStorageEnv(primary, secondary, tertiary, defaultValue string) string {
	if value := getEnv(primary, ""); value != "" {
		return value
	}
	if secondary != "" {
		if value := getEnv(secondary, ""); value != "" {
			return value
		}
	}
	if tertiary != "" {
		if value := getEnv(tertiary, ""); value != "" {
			return value
		}
	}
	return defaultValue
}

// InitLogger initializes the zerolog logger
func InitLogger() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}
