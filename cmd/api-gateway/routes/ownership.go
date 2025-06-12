package routes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
)

// PackageOwnershipRoutes sets up package ownership management routes
func PackageOwnershipRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	ownership := api.Group("/packages")
	ownership.Use(middleware.AuthMiddleware(authService))

	// Package ownership management
	ownership.GET("/:registry/:package/owners", handleGetPackageOwners(registryService))
	ownership.POST("/:registry/:package/owners", handleAddPackageOwner(registryService))
	ownership.DELETE("/:registry/:package/owners/:userId", handleRemovePackageOwner(registryService))

	// User's packages
	ownership.GET("/my-packages", handleGetUserPackages(registryService))
}

// AddOwnerRequest represents a request to add an owner
type AddOwnerRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Role   string    `json:"role" binding:"required"`
}

// GetPackageOwners godoc
//
//	@Summary		Get package owners
//	@Description	Retrieve all owners of a specific package
//	@Tags			Package Ownership
//	@Produce		json
//	@Param			registry	path		string	true	"Registry type (e.g., npm, nuget, maven)"
//	@Param			package		path		string	true	"Package name"
//	@Success		200			{object}	object{owners=[]object}	"Package owners retrieved successfully"
//	@Failure		401			{object}	object{error=string}	"Unauthorized"
//	@Failure		404			{object}	object{error=string}	"Package not found"
//	@Failure		500			{object}	object{error=string}	"Failed to retrieve package owners"
//	@Security		BearerAuth
//	@Router			/packages/{registry}/{package}/owners [get]
func handleGetPackageOwners(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryType := c.Param("registry")
		packageName := c.Param("package")

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		log.Info().
			Str("request_id", requestID).
			Str("registry", registryType).
			Str("package", packageName).
			Str("endpoint", "GET /packages/:registry/:package/owners").
			Msg("Get package owners request")

		// Get user from context
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			log.Warn().
				Str("request_id", requestID).
				Msg("User not found in context")
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "unauthorized",
			})
			return
		}

		// Check if user can view ownership (for now, any authenticated user can view)
		// In production, you might want to restrict this
		owners, err := registryService.GetPackageOwners(c.Request.Context(), registryType, packageName)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Err(err).
				Msg("Failed to get package owners")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "failed to get package owners",
			})
			return
		}

		log.Info().
			Str("request_id", requestID).
			Str("user_id", user.ID.String()).
			Int("owner_count", len(owners)).
			Msg("Package owners retrieved successfully")

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data:    owners,
		})
	}
}

// handleAddPackageOwner adds a new owner to a package
func handleAddPackageOwner(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryType := c.Param("registry")
		packageName := c.Param("package")

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		log.Info().
			Str("request_id", requestID).
			Str("registry", registryType).
			Str("package", packageName).
			Str("endpoint", "POST /packages/:registry/:package/owners").
			Msg("Add package owner request")

		// Get user from context
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			log.Warn().
				Str("request_id", requestID).
				Msg("User not found in context")
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "unauthorized",
			})
			return
		}

		// Parse request body
		var req AddOwnerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Warn().
				Str("request_id", requestID).
				Err(err).
				Msg("Invalid add owner request body")
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "invalid request body: " + err.Error(),
			})
			return
		}

		// Validate role
		if req.Role != "owner" && req.Role != "maintainer" && req.Role != "contributor" {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "invalid role: must be owner, maintainer, or contributor",
			})
			return
		}

		// Add owner
		err := registryService.AddPackageOwner(c.Request.Context(), registryType, packageName, user.ID, req.UserID, req.Role)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Err(err).
				Msg("Failed to add package owner")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		log.Info().
			Str("request_id", requestID).
			Str("granting_user_id", user.ID.String()).
			Str("target_user_id", req.UserID.String()).
			Str("role", req.Role).
			Msg("Package owner added successfully")

		c.JSON(http.StatusCreated, types.APIResponse{
			Success: true,
			Message: "package owner added successfully",
		})
	}
}

// handleRemovePackageOwner removes an owner from a package
func handleRemovePackageOwner(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		registryType := c.Param("registry")
		packageName := c.Param("package")
		userIDStr := c.Param("userId")

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		log.Info().
			Str("request_id", requestID).
			Str("registry", registryType).
			Str("package", packageName).
			Str("target_user_id", userIDStr).
			Str("endpoint", "DELETE /packages/:registry/:package/owners/:userId").
			Msg("Remove package owner request")

		// Get user from context
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			log.Warn().
				Str("request_id", requestID).
				Msg("User not found in context")
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "unauthorized",
			})
			return
		}

		// Parse target user ID
		targetUserID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "invalid user ID format",
			})
			return
		}

		// Remove owner
		err = registryService.RemovePackageOwner(c.Request.Context(), registryType, packageName, user.ID, targetUserID)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Err(err).
				Msg("Failed to remove package owner")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		log.Info().
			Str("request_id", requestID).
			Str("removing_user_id", user.ID.String()).
			Str("target_user_id", targetUserID.String()).
			Msg("Package owner removed successfully")

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "package owner removed successfully",
		})
	}
}

// handleGetUserPackages returns all packages a user has access to
func handleGetUserPackages(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		log.Info().
			Str("request_id", requestID).
			Str("endpoint", "GET /packages/my-packages").
			Msg("Get user packages request")

		// Get user from context
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			log.Warn().
				Str("request_id", requestID).
				Msg("User not found in context")
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "unauthorized",
			})
			return
		}

		// Get pagination parameters
		page := 1
		perPage := 50
		if pageStr := c.Query("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		if perPageStr := c.Query("per_page"); perPageStr != "" {
			if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
				perPage = pp
			}
		}

		// Get user packages
		packages, err := registryService.Ownership.GetUserPackages(c.Request.Context(), user.ID)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Err(err).
				Msg("Failed to get user packages")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "failed to get user packages",
			})
			return
		}

		// Simple pagination (in production, you'd want database-level pagination)
		total := len(packages)
		start := (page - 1) * perPage
		end := start + perPage
		if start >= total {
			packages = []types.PackageOwnership{}
		} else {
			if end > total {
				end = total
			}
			packages = packages[start:end]
		}

		totalPages := (total + perPage - 1) / perPage
		pagination := &types.PaginationInfo{
			Page:       page,
			PerPage:    perPage,
			Total:      int64(total),
			TotalPages: totalPages,
		}

		log.Info().
			Str("request_id", requestID).
			Str("user_id", user.ID.String()).
			Int("package_count", len(packages)).
			Int("total_packages", total).
			Msg("User packages retrieved successfully")

		c.JSON(http.StatusOK, types.PaginatedResponse{
			APIResponse: types.APIResponse{
				Success: true,
				Data:    packages,
			},
			Pagination: pagination,
		})
	}
}
