package config

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	Proxy    ProxyConfig    `yaml:"proxy"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host               string        `yaml:"host"`
	Port               int           `yaml:"port"`
	ReadTimeout        time.Duration `yaml:"read_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout"`
	IdleTimeout        time.Duration `yaml:"idle_timeout"`
	CORSAllowedOrigins []string      `yaml:"cors_allowed_origins"`
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
	JWTSecret           string        `yaml:"jwt_secret"`
	JWTExpiration       time.Duration `yaml:"jwt_expiration"`
	BCryptCost          int           `yaml:"bcrypt_cost"`
	UISessionCookieName string        `yaml:"ui_session_cookie_name"`
	UISessionExpiration time.Duration `yaml:"ui_session_expiration"`
	UICookieSecure      bool          `yaml:"ui_cookie_secure"`
	UICookieSameSite    string        `yaml:"ui_cookie_same_site"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json, text
}

// ProxyConfig holds upstream proxy/cache configuration
type ProxyConfig struct {
	Enabled          bool                  `yaml:"enabled"`
	TimeoutSeconds   int                   `yaml:"timeoutSeconds"`
	MaxArtifactBytes int64                 `yaml:"maxArtifactBytes"`
	Registries       ProxyRegistriesConfig `yaml:"registries"`
}

// ProxyRegistriesConfig holds per-registry upstream settings
type ProxyRegistriesConfig struct {
	NPM      ProxyRegistryConfig `yaml:"npm"`
	NuGet    ProxyRegistryConfig `yaml:"nuget"`
	Maven    ProxyRegistryConfig `yaml:"maven"`
	Go       ProxyRegistryConfig `yaml:"go"`
	Helm     ProxyRegistryConfig `yaml:"helm"`
	Cargo    ProxyRegistryConfig `yaml:"cargo"`
	RubyGems ProxyRegistryConfig `yaml:"rubygems"`
	OPA      ProxyRegistryConfig `yaml:"opa"`
	OCI      ProxyRegistryConfig `yaml:"oci"`
}

// ProxyRegistryConfig holds upstream settings for one registry family
type ProxyRegistryConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Upstream string `yaml:"upstream"`
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() *Config {
	return &Config{
		Server: ServerConfig{
			Host:               getEnv("SERVER_HOST", "0.0.0.0"),
			Port:               getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:        getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:       getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:        getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
			CORSAllowedOrigins: getEnvStringSlice("CORS_ALLOWED_ORIGINS"),
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
			JWTSecret:           getEnv("JWT_SECRET", "your-secret-key"),
			JWTExpiration:       getEnvDuration("JWT_EXPIRATION", 24*time.Hour),
			BCryptCost:          getEnvInt("BCRYPT_COST", 12),
			UISessionCookieName: getEnv("UI_SESSION_COOKIE_NAME", "lodestone_ui_session"),
			UISessionExpiration: getEnvDuration("UI_SESSION_EXPIRATION", 8*time.Hour),
			UICookieSecure:      getEnvBool("UI_COOKIE_SECURE", true),
			UICookieSameSite:    getEnv("UI_COOKIE_SAME_SITE", "Lax"),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Proxy: ProxyConfig{
			Enabled:          getEnvBool("PROXY_ENABLED", false),
			TimeoutSeconds:   getEnvInt("PROXY_TIMEOUT_SECONDS", 30),
			MaxArtifactBytes: getEnvInt64("PROXY_MAX_ARTIFACT_BYTES", 0),
			Registries: ProxyRegistriesConfig{
				NPM: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_NPM_ENABLED", true),
					Upstream: getEnv("PROXY_NPM_UPSTREAM", "https://registry.npmjs.org"),
				},
				NuGet: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_NUGET_ENABLED", true),
					Upstream: getEnv("PROXY_NUGET_UPSTREAM", "https://api.nuget.org/v3/index.json"),
				},
				Maven: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_MAVEN_ENABLED", true),
					Upstream: getEnv("PROXY_MAVEN_UPSTREAM", "https://repo1.maven.org/maven2"),
				},
				Go: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_GO_ENABLED", true),
					Upstream: getEnv("PROXY_GO_UPSTREAM", "https://proxy.golang.org"),
				},
				Helm: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_HELM_ENABLED", true),
					Upstream: getEnv("PROXY_HELM_UPSTREAM", "https://charts.bitnami.com/bitnami"),
				},
				Cargo: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_CARGO_ENABLED", true),
					Upstream: getEnv("PROXY_CARGO_UPSTREAM", "https://crates.io"),
				},
				RubyGems: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_RUBYGEMS_ENABLED", true),
					Upstream: getEnv("PROXY_RUBYGEMS_UPSTREAM", "https://rubygems.org"),
				},
				OPA: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_OPA_ENABLED", true),
					Upstream: getEnv("PROXY_OPA_UPSTREAM", ""),
				},
				OCI: ProxyRegistryConfig{
					Enabled:  getEnvBool("PROXY_OCI_ENABLED", true),
					Upstream: getEnv("PROXY_OCI_UPSTREAM", "https://registry-1.docker.io"),
				},
			},
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

// UISameSite returns the configured browser session cookie SameSite mode.
func (c *AuthConfig) UISameSite() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(c.UICookieSameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		fallthrough
	default:
		return http.SameSiteLaxMode
	}
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

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
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
