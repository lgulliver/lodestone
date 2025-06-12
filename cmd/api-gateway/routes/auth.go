package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
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

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Create a new user account in the system
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			user	body		types.RegisterRequest	true	"User registration information"
//	@Success		201		{object}	object{user=object{id=string,username=string,email=string}}	"User created successfully"
//	@Failure		400		{object}	object{error=string}	"Invalid request body"
//	@Failure		500		{object}	object{error=string}	"Registration failed"
//	@Router			/auth/register [post]
func handleRegister(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		log.Info().
			Str("request_id", requestID).
			Str("endpoint", "POST /auth/register").
			Str("client_ip", c.ClientIP()).
			Msg("Registration request received")

		var req types.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Warn().
				Str("request_id", requestID).
				Err(err).
				Msg("Invalid registration request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "request_id", requestID)

		user, err := authService.Register(ctx, &req)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("username", req.Username).
				Err(err).
				Msg("Registration failed")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Info().
			Str("request_id", requestID).
			Str("username", user.Username).
			Str("user_id", user.ID.String()).
			Msg("Registration successful")

		c.JSON(http.StatusCreated, gin.H{
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
			},
		})
	}
}

// Login godoc
//
//	@Summary		User login
//	@Description	Authenticate user and return JWT token
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			credentials	body		types.LoginRequest	true	"User login credentials"
//	@Success		200			{object}	object{token=string,user=object{id=string}}	"Login successful"
//	@Failure		400			{object}	object{error=string}	"Invalid request body"
//	@Failure		401			{object}	object{error=string}	"Invalid credentials"
//	@Router			/auth/login [post]
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

// CreateAPIKey godoc
//
//	@Summary		Create a new API key
//	@Description	Generate a new API key for the authenticated user
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			api_key	body		object{name=string,permissions=[]string}	true	"API key creation request"
//	@Success		201		{object}	object{api_key=object{},key=string}	"API key created successfully"
//	@Failure		400		{object}	object{error=string}	"Invalid request body"
//	@Failure		401		{object}	object{error=string}	"Unauthorized"
//	@Failure		500		{object}	object{error=string}	"Failed to create API key"
//	@Security		BearerAuth
//	@Router			/auth/api-keys [post]
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

// ListAPIKeys godoc
//
//	@Summary		List user's API keys
//	@Description	Get all API keys for the authenticated user
//	@Tags			Authentication
//	@Produce		json
//	@Success		200	{object}	object{api_keys=[]object}	"List of API keys"
//	@Failure		401	{object}	object{error=string}	"Unauthorized"
//	@Failure		500	{object}	object{error=string}	"Failed to list API keys"
//	@Security		BearerAuth
//	@Router			/auth/api-keys [get]
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

// RevokeAPIKey godoc
//
//	@Summary		Revoke an API key
//	@Description	Delete an API key for the authenticated user
//	@Tags			Authentication
//	@Produce		json
//	@Param			id	path		string	true	"API Key ID"
//	@Success		200	{object}	object{message=string}	"API key revoked successfully"
//	@Failure		400	{object}	object{error=string}	"Invalid API key ID"
//	@Failure		401	{object}	object{error=string}	"Unauthorized"
//	@Failure		500	{object}	object{error=string}	"Failed to revoke API key"
//	@Security		BearerAuth
//	@Router			/auth/api-keys/{id} [delete]
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
