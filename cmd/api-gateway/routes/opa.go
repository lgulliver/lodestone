package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// OPARoutes sets up Open Policy Agent bundle repository routes
func OPARoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	opa := api.Group("/opa")

	// OPA bundle API
	opa.GET("/bundles/:name", middleware.OptionalAuthMiddleware(authService), handleOPABundleDownload(registryService))
	opa.GET("/bundles/:name/:version", middleware.OptionalAuthMiddleware(authService), handleOPABundleVersionDownload(registryService))
	opa.GET("/bundles", handleOPABundleList(registryService))

	// Bundle upload (requires authentication)
	opa.PUT("/bundles/:name", middleware.AuthMiddleware(authService), handleOPABundleUpload(registryService))
	opa.PUT("/bundles/:name/:version", middleware.AuthMiddleware(authService), handleOPABundleVersionUpload(registryService))
	opa.DELETE("/bundles/:name/:version", middleware.AuthMiddleware(authService), handleOPABundleDelete(registryService))
}

func handleOPABundleDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		bundleName := c.Param("name")
		if bundleName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bundle name required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "opa")

		// Get the latest version of the bundle
		filter := &types.ArtifactFilter{
			Name:     bundleName,
			Registry: "opa",
			Limit:    1,
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get bundle"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "bundle not found"})
			return
		}

		artifact := artifacts[0]
		_, content, err := registryService.Download(ctx, "opa", bundleName, artifact.Version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "bundle not found"})
			return
		}

		c.Header("Content-Type", "application/gzip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.tar.gz", bundleName))
		c.Header("ETag", fmt.Sprintf(`"%s"`, artifact.SHA256))

		if artifact.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream bundle"})
			return
		}
	}
}

func handleOPABundleVersionDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		bundleName := c.Param("name")
		version := c.Param("version")

		if bundleName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bundle name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "opa")

		artifact, content, err := registryService.Download(ctx, "opa", bundleName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "bundle version not found"})
			return
		}

		c.Header("Content-Type", "application/gzip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.tar.gz", bundleName, version))
		c.Header("ETag", fmt.Sprintf(`"%s"`, artifact.SHA256))

		if artifact.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream bundle"})
			return
		}
	}
}

func handleOPABundleList(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "registry", "opa")

		filter := &types.ArtifactFilter{
			Registry: "opa",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list bundles"})
			return
		}

		// Group bundles by name and show latest version info
		bundles := make(map[string]gin.H)
		for _, artifact := range artifacts {
			bundleInfo, exists := bundles[artifact.Name]
			if !exists {
				var description string
				if artifact.Metadata != nil {
					if desc, ok := artifact.Metadata["description"].(string); ok {
						description = desc
					}
				}

				bundles[artifact.Name] = gin.H{
					"name":        artifact.Name,
					"version":     artifact.Version,
					"description": description,
					"created_at":  artifact.CreatedAt,
					"size":        artifact.Size,
				}
			} else {
				// Compare timestamps to get the latest version
				if existingTime, ok := bundleInfo["created_at"].(time.Time); ok {
					if artifact.CreatedAt.After(existingTime) {
						var description string
						if artifact.Metadata != nil {
							if desc, ok := artifact.Metadata["description"].(string); ok {
								description = desc
							}
						}

						bundles[artifact.Name] = gin.H{
							"name":        artifact.Name,
							"version":     artifact.Version,
							"description": description,
							"created_at":  artifact.CreatedAt,
							"size":        artifact.Size,
						}
					}
				}
			}
		}

		// Convert map to slice
		bundleList := make([]gin.H, 0, len(bundles))
		for _, bundle := range bundles {
			bundleList = append(bundleList, bundle)
		}

		c.JSON(http.StatusOK, gin.H{
			"bundles": bundleList,
			"total":   len(bundleList),
		})
	}
}

func handleOPABundleUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		bundleName := c.Param("name")
		if bundleName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bundle name required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "opa")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// Use current timestamp as version if not specified
		versionHeader := c.Request.Header.Get("X-Bundle-Version")
		version := "latest"
		if versionHeader != "" {
			version = fmt.Sprintf("v%s", versionHeader)
		}

		_, err := registryService.Upload(ctx, "opa", bundleName, version, c.Request.Body, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "bundle uploaded successfully",
			"name":    bundleName,
			"version": version,
		})
	}
}

func handleOPABundleVersionUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		bundleName := c.Param("name")
		version := c.Param("version")

		if bundleName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bundle name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "opa")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		_, err := registryService.Upload(ctx, "opa", bundleName, version, c.Request.Body, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "bundle uploaded successfully",
			"name":    bundleName,
			"version": version,
		})
	}
}

func handleOPABundleDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		bundleName := c.Param("name")
		version := c.Param("version")

		if bundleName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bundle name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "opa")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, "opa", bundleName, version, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete bundle"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "bundle deleted successfully",
		})
	}
}
