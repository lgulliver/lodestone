package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/pkg/types"
)

// AuthRoutes sets up authentication-related routes
func AuthRoutes(api *gin.RouterGroup, authService *auth.Service) {
	auth := api.Group("/auth")

	// Public routes
	auth.POST("/register", handleRegister(authService))
	auth.POST("/login", handleLogin(authService))

	// Protected routes
	authenticated := auth.Group("/")
	authenticated.Use(middleware.AuthMiddleware(authService))
	authenticated.POST("/api-keys", handleCreateAPIKey(authService))
	authenticated.GET("/api-keys", handleListAPIKeys(authService))
	authenticated.DELETE("/api-keys/:id", handleRevokeAPIKey(authService))
}

func handleRegister(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "request_id", c.GetHeader("X-Request-ID"))

		user, err := authService.Register(ctx, &req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
			},
		})
	}
}

func handleLogin(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "request_id", c.GetHeader("X-Request-ID"))

		authToken, err := authService.Login(ctx, &req)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": authToken.Token,
			"user": gin.H{
				"id": authToken.UserID,
			},
		})
	}
}

func handleCreateAPIKey(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			Name        string   `json:"name" binding:"required"`
			Permissions []string `json:"permissions"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "user_id", user.ID)

		apiKey, keyValue, err := authService.CreateAPIKey(ctx, user.ID, req.Name, req.Permissions)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"api_key": apiKey,
			"key":     keyValue,
		})
	}
}

func handleListAPIKeys(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "user_id", user.ID)

		apiKeys, err := authService.ListAPIKeys(ctx, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"api_keys": apiKeys,
		})
	}
}

func handleRevokeAPIKey(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		keyID := c.Param("id")
		if keyID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "API key ID required"})
			return
		}

		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid API key ID format"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "user_id", user.ID)

		err = authService.RevokeAPIKey(ctx, keyUUID, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke API key"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "API key revoked successfully",
		})
	}
}
