package middleware

import (
	"net/http"
	"strings"

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
		return normalizeRegistryType(registry)
	}

	// Check if registry is specified as a query parameter
	if registry := c.Query("registry"); registry != "" {
		return normalizeRegistryType(registry)
	}

	path := strings.Trim(c.Request.URL.Path, "/")
	if path == "" {
		return ""
	}

	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return ""
	}

	// Root OCI/Docker API routes.
	if segments[0] == "v2" {
		return "oci"
	}

	// API routes are mounted under /api/v1.
	if len(segments) >= 3 && segments[0] == "api" && segments[1] == "v1" {
		if segments[2] == "v2" {
			return "oci"
		}
		return normalizeRegistryType(segments[2])
	}

	return ""
}

func normalizeRegistryType(registryType string) string {
	registryType = strings.ToLower(strings.TrimSpace(registryType))

	switch registryType {
	case "nuget", "npm", "maven", "cargo", "helm", "rubygems", "opa", "go", "oci":
		return registryType
	case "docker":
		return "oci"
	case "gems":
		return "rubygems"
	default:
		return ""
	}
}
