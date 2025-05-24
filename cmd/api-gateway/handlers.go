package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
)

// Authentication middleware
func authMiddleware(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "Missing authorization header",
			})
			c.Abort()
			return
		}

		var user *types.User
		var err error

		// Check if it's a Bearer token (JWT)
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			user, err = authService.ValidateToken(c.Request.Context(), token)
		} else if strings.HasPrefix(authHeader, "ApiKey ") {
			// API Key authentication
			apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
			user, _, err = authService.ValidateAPIKey(c.Request.Context(), apiKey)
		} else {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "Invalid authorization format",
			})
			c.Abort()
			return
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "Invalid credentials",
			})
			c.Abort()
			return
		}

		// Store user in context
		c.Set("user", user)
		c.Next()
	}
}

// Auth handlers
func handleRegister(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Invalid request format",
			})
			return
		}

		user, err := authService.Register(c.Request.Context(), &req)
		if err != nil {
			c.JSON(http.StatusConflict, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, types.APIResponse{
			Success: true,
			Message: "User registered successfully",
			Data:    user,
		})
	}
}

func handleLogin(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Invalid request format",
			})
			return
		}

		token, err := authService.Login(c.Request.Context(), &req)
		if err != nil {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "Login successful",
			Data:    token,
		})
	}
}

func handleCreateAPIKey(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*types.User)

		var req struct {
			Name        string   `json:"name" binding:"required"`
			Permissions []string `json:"permissions"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Invalid request format",
			})
			return
		}

		apiKey, keyValue, err := authService.CreateAPIKey(c.Request.Context(), user.ID, req.Name, req.Permissions)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, types.APIResponse{
			Success: true,
			Message: "API key created successfully",
			Data: map[string]interface{}{
				"api_key": apiKey,
				"key":     keyValue, // Only shown once
			},
		})
	}
}

func handleListAPIKeys(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*types.User)

		apiKeys, err := authService.ListAPIKeys(c.Request.Context(), user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    apiKeys,
		})
	}
}

func handleRevokeAPIKey(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*types.User)
		keyID := c.Param("id")

		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Invalid API key ID",
			})
			return
		}

		if err := authService.RevokeAPIKey(c.Request.Context(), keyUUID, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "API key revoked successfully",
		})
	}
}

// Registry handlers
func handleListPackages(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryType := c.Param("type")
		
		// Parse query parameters
		name := c.Query("name")
		limitStr := c.DefaultQuery("limit", "50")
		offsetStr := c.DefaultQuery("offset", "0")

		limit, _ := strconv.Atoi(limitStr)
		offset, _ := strconv.Atoi(offsetStr)

		filter := &types.ArtifactFilter{
			Name:     name,
			Registry: registryType,
			Limit:    limit,
			Offset:   offset,
		}

		artifacts, total, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		// Calculate pagination info
		totalPages := (int(total) + limit - 1) / limit
		page := (offset / limit) + 1

		response := types.PaginatedResponse{
			APIResponse: types.APIResponse{
				Success: true,
				Data:    artifacts,
			},
			Pagination: &types.PaginationInfo{
				Page:       page,
				PerPage:    limit,
				Total:      total,
				TotalPages: totalPages,
			},
		}

		c.JSON(http.StatusOK, response)
	}
}

func handleUploadPackage(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*types.User)
		registryType := c.Param("type")
		name := c.Param("name")
		version := c.Param("version")

		// Validate registry type
		if !utils.IsValidRegistryType(registryType) {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Unsupported registry type",
			})
			return
		}

		// Get file from multipart form
		file, header, err := c.Request.FormFile("package")
		if err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "No package file provided",
			})
			return
		}
		defer file.Close()

		logrus.Infof("Uploading package: %s/%s:%s (size: %d bytes)", registryType, name, version, header.Size)

		artifact, err := registryService.Upload(c.Request.Context(), registryType, name, version, file, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, types.APIResponse{
			Success: true,
			Message: "Package uploaded successfully",
			Data:    artifact,
		})
	}
}

func handleDownloadPackage(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryType := c.Param("type")
		name := c.Param("name")
		version := c.Param("version")

		artifact, content, err := registryService.Download(c.Request.Context(), registryType, name, version)
		if err != nil {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		defer content.Close()

		// Set appropriate headers
		c.Header("Content-Type", artifact.ContentType)
		c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
		c.Header("Content-Disposition", "attachment; filename=\""+artifact.Name+"-"+artifact.Version+"\"")
		c.Header("X-Checksum-SHA256", artifact.SHA256)

		// Stream the content
		if _, err := io.Copy(c.Writer, content); err != nil {
			logrus.Errorf("Failed to stream package content: %v", err)
		}
	}
}

func handleDeletePackage(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*types.User)
		registryType := c.Param("type")
		name := c.Param("name")
		version := c.Param("version")

		if err := registryService.Delete(c.Request.Context(), registryType, name, version, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "Package deleted successfully",
		})
	}
}
