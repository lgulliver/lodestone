package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
)

const uiCookiePath = "/api/v1/ui"

// UIRoutes sets up browser-facing routes that rely on secure cookies instead of exposing tokens.
func UIRoutes(api *gin.RouterGroup, authService *auth.Service, authConfig *config.AuthConfig) {
	ui := api.Group("/ui")
	ui.POST("/auth/login", handleUILogin(authService, authConfig))

	authenticated := ui.Group("/")
	authenticated.Use(middleware.UIAuthMiddleware(authService, authConfig))
	authenticated.GET("/auth/session", handleUISession(authConfig))
	authenticated.POST("/auth/logout", handleUILogout(authConfig))
	authenticated.POST("/auth/api-keys", handleCreateAPIKey(authService))
	authenticated.GET("/auth/api-keys", handleListAPIKeys(authService))
	authenticated.DELETE("/auth/api-keys/:id", handleRevokeAPIKey(authService))
}

func handleUILogin(authService *auth.Service, authConfig *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := context.WithValue(c.Request.Context(), requestIDContextKey, c.GetHeader("X-Request-ID"))
		authToken, err := authService.LoginWithExpiration(ctx, &req, authConfig.UISessionExpiration)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		user, err := authService.GetUserByID(ctx, authToken.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		cookieTTL := int(time.Until(authToken.ExpiresAt).Seconds())
		if cookieTTL < 0 {
			cookieTTL = 0
		}
		csrfToken := middleware.BuildCSRFSignedToken(authToken.Token, authConfig.JWTSecret)
		setNoStore(c)
		setBrowserSessionCookie(c, authConfig, authToken.Token, cookieTTL)
		c.Header("X-CSRF-Token", csrfToken)

		c.JSON(http.StatusOK, gin.H{
			"user": buildUserResponse(user),
		})
	}
}

func handleUISession(authConfig *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		setNoStore(c)
		if sessionToken, err := c.Cookie(authConfig.UISessionCookieName); err == nil {
			c.Header("X-CSRF-Token", middleware.BuildCSRFSignedToken(sessionToken, authConfig.JWTSecret))
		}
		c.JSON(http.StatusOK, gin.H{
			"user": buildUserResponse(user),
		})
	}
}

func handleUILogout(authConfig *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		setNoStore(c)
		clearBrowserSessionCookie(c, authConfig)
		c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
	}
}

func buildUserResponse(user *types.User) gin.H {
	return gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"is_active":  user.IsActive,
		"is_admin":   user.IsAdmin,
		"created_at": user.CreatedAt,
	}
}

func setBrowserSessionCookie(c *gin.Context, authConfig *config.AuthConfig, value string, maxAge int) {
	c.SetSameSite(authConfig.UISameSite())
	c.SetCookie(authConfig.UISessionCookieName, value, maxAge, uiCookiePath, "", authConfig.UICookieSecure, true)
}

func clearBrowserSessionCookie(c *gin.Context, authConfig *config.AuthConfig) {
	setBrowserSessionCookie(c, authConfig, "", -1)
}

func setNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}
