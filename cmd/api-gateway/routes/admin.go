package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
)

// AdminRoutes sets up the admin API routes for registry management
func AdminRoutes(r *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	// Create settings service
	settingsService := registry.NewRegistrySettingsService(registryService.DB.DB)

	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware(authService))
	admin.Use(adminOnlyMiddleware())

	// Registry management endpoints
	registries := admin.Group("/registries")
	{
		registries.GET("/", getRegistrySettings(settingsService))
		registries.GET("/:registry", getRegistrySetting(settingsService))
		registries.PUT("/:registry/enable", enableRegistry(settingsService))
		registries.PUT("/:registry/disable", disableRegistry(settingsService))
		registries.PUT("/:registry/description", updateRegistryDescription(settingsService))
	}
}

// adminOnlyMiddleware ensures only admin users can access admin endpoints
func adminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "Authentication required",
			})
			c.Abort()
			return
		}

		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, types.APIResponse{
				Success: false,
				Error:   "Admin privileges required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getRegistrySettings returns all registry settings
func getRegistrySettings(settingsService *registry.RegistrySettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := settingsService.GetRegistrySettings(c.Request.Context())
		if err != nil {
			log.Error().Err(err).Msg("failed to get registry settings")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "Failed to retrieve registry settings",
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    settings,
		})
	}
}

// getRegistrySetting returns a specific registry setting
func getRegistrySetting(settingsService *registry.RegistrySettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryName := c.Param("registry")

		setting, err := settingsService.GetRegistrySetting(c.Request.Context(), registryName)
		if err != nil {
			log.Error().Err(err).Str("registry", registryName).Msg("failed to get registry setting")
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   "Registry not found",
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    setting,
		})
	}
}

// enableRegistry enables a registry format
func enableRegistry(settingsService *registry.RegistrySettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryName := c.Param("registry")
		user, _ := middleware.GetUserFromContext(c)

		err := settingsService.EnableRegistry(c.Request.Context(), registryName, user.ID)
		if err != nil {
			log.Error().Err(err).Str("registry", registryName).Msg("failed to enable registry")
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "Registry enabled successfully",
		})
	}
}

// disableRegistry disables a registry format
func disableRegistry(settingsService *registry.RegistrySettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryName := c.Param("registry")
		user, _ := middleware.GetUserFromContext(c)

		err := settingsService.DisableRegistry(c.Request.Context(), registryName, user.ID)
		if err != nil {
			log.Error().Err(err).Str("registry", registryName).Msg("failed to disable registry")
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "Registry disabled successfully",
		})
	}
}

// updateRegistryDescription updates the description of a registry
func updateRegistryDescription(settingsService *registry.RegistrySettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryName := c.Param("registry")
		user, _ := middleware.GetUserFromContext(c)

		var request struct {
			Description string `json:"description" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Invalid request body",
			})
			return
		}

		err := settingsService.UpdateRegistryDescription(c.Request.Context(), registryName, request.Description, user.ID)
		if err != nil {
			log.Error().Err(err).Str("registry", registryName).Msg("failed to update registry description")
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "Registry description updated successfully",
		})
	}
}
