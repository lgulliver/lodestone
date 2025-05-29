package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
)

// AuthMiddleware validates JWT tokens and API keys
func AuthMiddleware(authService *auth.Service) gin.HandlerFunc {
	return authMiddlewareWithInterface(authService)
}

// authMiddlewareWithInterface is the testable version that accepts an interface
func authMiddlewareWithInterface(authService AuthServiceInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for JWT token in Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				ctx := context.WithValue(c.Request.Context(), "token", token)

				// First try to validate as JWT token
				user, err := authService.ValidateToken(ctx, token)
				if err == nil {
					c.Set("user", user)
					c.Next()
					return
				}

				log.Debug().Err(err).Str("path", c.Request.URL.Path).Msg("JWT token validation failed, trying API key")

				// Fall back to API key validation for Bearer tokens (Docker CLI compatibility)
				ctx = context.WithValue(c.Request.Context(), "api_key", token)
				user, _, err = authService.ValidateAPIKey(ctx, token)
				if err == nil {
					log.Debug().Str("username", user.Username).Msg("API key validation successful")
					c.Set("user", user)
					c.Next()
					return
				}

				log.Warn().Err(err).Str("path", c.Request.URL.Path).Msg("Both JWT and API key validation failed")

				// For OCI/Docker endpoints, return proper WWW-Authenticate header
				if strings.HasPrefix(c.Request.URL.Path, "/v2/") {
					c.Header("WWW-Authenticate", `Bearer realm="http://localhost:8080/v2/auth",service="registry",scope="repository:*:*"`)
				}

				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				c.Abort()
				return
			}
		}

		// Check for API key in X-API-Key header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			log.Debug().Str("path", c.Request.URL.Path).Msg("Validating API key from header")
			ctx := context.WithValue(c.Request.Context(), "api_key", apiKey)

			user, _, err := authService.ValidateAPIKey(ctx, apiKey)
			if err == nil {
				log.Debug().Str("username", user.Username).Msg("API key validation successful")
				c.Set("user", user)
				c.Next()
				return
			}
			log.Warn().Err(err).Msg("API key validation failed")
		}

		// Check for API key in X-NuGet-ApiKey header (NuGet specific)
		nugetApiKey := c.GetHeader("X-NuGet-ApiKey")
		if nugetApiKey != "" {
			log.Debug().Str("path", c.Request.URL.Path).Msg("Validating NuGet API key from header")
			ctx := context.WithValue(c.Request.Context(), "api_key", nugetApiKey)

			user, _, err := authService.ValidateAPIKey(ctx, nugetApiKey)
			if err == nil {
				log.Debug().Str("username", user.Username).Msg("NuGet API key validation successful")
				c.Set("user", user)
				c.Next()
				return
			}
			log.Warn().Err(err).Msg("NuGet API key validation failed")
		}

		// Check for API key in query parameter (for some package managers)
		if apiKey := c.Query("api_key"); apiKey != "" {
			log.Debug().Str("path", c.Request.URL.Path).Msg("Validating API key from query parameter")
			ctx := context.WithValue(c.Request.Context(), "api_key", apiKey)

			user, _, err := authService.ValidateAPIKey(ctx, apiKey)
			if err == nil {
				log.Debug().Str("username", user.Username).Msg("API key validation successful")
				c.Set("user", user)
				c.Next()
				return
			}
			log.Warn().Err(err).Msg("API key validation failed")
		}

		log.Warn().
			Str("path", c.Request.URL.Path).
			Str("client_ip", c.ClientIP()).
			Msg("Unauthorized access attempt")

		// For OCI/Docker endpoints, return proper WWW-Authenticate header
		if strings.HasPrefix(c.Request.URL.Path, "/v2/") {
			c.Header("WWW-Authenticate", `Bearer realm="http://localhost:8080/v2/auth",service="registry",scope="repository:*:*"`)
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		c.Abort()
	}
}

// OptionalAuthMiddleware allows both authenticated and anonymous access
func OptionalAuthMiddleware(authService *auth.Service) gin.HandlerFunc {
	return optionalAuthMiddlewareWithInterface(authService)
}

// optionalAuthMiddlewareWithInterface is the testable version that accepts an interface
func optionalAuthMiddlewareWithInterface(authService AuthServiceInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to authenticate, but don't fail if no auth provided
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			ctx := context.WithValue(c.Request.Context(), "token", token)

			// First try to validate as JWT token
			if user, err := authService.ValidateToken(ctx, token); err == nil {
				c.Set("user", user)
			} else {
				// Fall back to API key validation for Bearer tokens (Docker CLI compatibility)
				ctx := context.WithValue(c.Request.Context(), "api_key", token)
				if user, _, err := authService.ValidateAPIKey(ctx, token); err == nil {
					c.Set("user", user)
				}
			}
			// For optional auth, we continue even if JWT validation fails
		} else if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			ctx := context.WithValue(c.Request.Context(), "api_key", apiKey)

			if user, _, err := authService.ValidateAPIKey(ctx, apiKey); err == nil {
				c.Set("user", user)
			}
		} else if apiKey := c.Query("api_key"); apiKey != "" {
			ctx := context.WithValue(c.Request.Context(), "api_key", apiKey)

			if user, _, err := authService.ValidateAPIKey(ctx, apiKey); err == nil {
				c.Set("user", user)
			}
		}

		c.Next()
	}
}

// GetUserFromContext extracts the authenticated user from gin context
func GetUserFromContext(c *gin.Context) (*types.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	typedUser, ok := user.(*types.User)
	return typedUser, ok
}
