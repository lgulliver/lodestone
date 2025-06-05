package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/rs/zerolog/log"
)

// RegistryValidationMiddleware checks if a registry is enabled before processing requests
func RegistryValidationMiddleware(settingsService *registry.RegistrySettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract registry type from the request path or parameters
		registryType := extractRegistryType(c)

		if registryType == "" {
			// If we can't determine the registry type, continue
			c.Next()
			return
		}

		// Check if the registry is enabled
		enabled, err := settingsService.IsRegistryEnabled(c.Request.Context(), registryType)
		if err != nil {
			log.Error().
				Err(err).
				Str("registry", registryType).
				Msg("failed to check registry status")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			c.Abort()
			return
		}

		if !enabled {
			log.Warn().
				Str("registry", registryType).
				Str("path", c.Request.URL.Path).
				Msg("request to disabled registry")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":    "Registry is currently disabled",
				"registry": registryType,
			})
			c.Abort()
			return
		}

		// Registry is enabled, continue with the request
		c.Next()
	}
}

// extractRegistryType extracts the registry type from the request
func extractRegistryType(c *gin.Context) string {
	// Check if registry is specified as a URL parameter
	if registry := c.Param("registry"); registry != "" {
		return registry
	}

	// Check if registry is specified as a query parameter
	if registry := c.Query("registry"); registry != "" {
		return registry
	}

	// Extract from path patterns
	path := c.Request.URL.Path

	// Common registry path patterns
	patterns := map[string]string{
		"/v1/nuget":    "nuget",
		"/v1/npm":      "npm",
		"/v1/maven":    "maven",
		"/v1/cargo":    "cargo",
		"/v1/docker":   "docker",
		"/v1/helm":     "helm",
		"/v1/rubygems": "rubygems",
		"/v1/opa":      "opa",
		"/v1/go":       "go",
		"/v2/":         "docker", // Docker registry v2 API
	}

	for pattern, registryType := range patterns {
		if containsPath(path, pattern) {
			return registryType
		}
	}

	return ""
}

// containsPath checks if the path contains the pattern
func containsPath(path, pattern string) bool {
	return len(path) >= len(pattern) && path[:len(pattern)] == pattern
}
