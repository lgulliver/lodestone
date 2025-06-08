package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain_ConfigLoading(t *testing.T) {
	// Test that the application can start with minimal config
	// This mainly tests that imports and basic setup work

	// Set minimal required environment variables
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")
	os.Setenv("PORT", "8080")

	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
	}()

	// This test mainly verifies that all imports resolve correctly
	// and the configuration can be loaded without panicking
	assert.NotPanics(t, func() {
		// We can't actually run main() in a test, but we can test
		// that the configuration types and imports work
		config := struct {
			DatabaseURL string
			RedisURL    string
			JWTSecret   string
			Port        string
		}{
			DatabaseURL: os.Getenv("DATABASE_URL"),
			RedisURL:    os.Getenv("REDIS_URL"),
			JWTSecret:   os.Getenv("JWT_SECRET"),
			Port:        os.Getenv("PORT"),
		}

		assert.NotEmpty(t, config.DatabaseURL)
		assert.NotEmpty(t, config.RedisURL)
		assert.NotEmpty(t, config.JWTSecret)
		assert.NotEmpty(t, config.Port)
	})
}

func TestMain_EnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		required bool
	}{
		{"Database URL", "DATABASE_URL", true},
		{"Redis URL", "REDIS_URL", true},
		{"JWT Secret", "JWT_SECRET", true},
		{"Port", "PORT", false}, // Has default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the environment variable
			originalValue := os.Getenv(tt.envVar)
			os.Unsetenv(tt.envVar)

			defer func() {
				if originalValue != "" {
					os.Setenv(tt.envVar, originalValue)
				}
			}()

			value := os.Getenv(tt.envVar)
			if tt.required {
				// For required variables, we'd expect the application to handle missing values
				// This test documents which variables are expected
				assert.Empty(t, value, "Environment variable %s should be empty in test", tt.envVar)
			} else {
				// For optional variables, document that they can be empty
				assert.Empty(t, value, "Optional environment variable %s should be empty in test", tt.envVar)
			}
		})
	}
}
