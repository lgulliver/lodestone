package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// NuGetRoutes sets up NuGet package manager routes
func NuGetRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	nuget := api.Group("/nuget")
	
	// NuGet v3 API routes
	// Service index (metadata endpoint)
	nuget.GET("/v3-flatcontainer/:id/index.json", handleNuGetServiceIndex(registryService))
	nuget.GET("/v3-flatcontainer/:id/:version/:filename", middleware.OptionalAuthMiddleware(authService), handleNuGetDownload(registryService))
	
	// Package publish (requires authentication)
	nuget.PUT("/api/v2/package", middleware.AuthMiddleware(authService), handleNuGetUpload(registryService))
	nuget.DELETE("/api/v2/package/:id/:version", middleware.AuthMiddleware(authService), handleNuGetDelete(registryService))
	
	// Search API
	nuget.GET("/v3/search", handleNuGetSearch(registryService))
	
	// Package metadata
	nuget.GET("/v3/registration/:id/index.json", handleNuGetPackageMetadata(registryService))
}

func handleNuGetServiceIndex(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		if packageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		
		filter := &types.ArtifactFilter{
			Name:     packageID,
			Registry: "nuget",
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list packages"})
			return
		}

		versions := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			versions = append(versions, artifact.Version)
		}

		c.JSON(http.StatusOK, gin.H{
			"versions": versions,
		})
	}
}

func handleNuGetDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		version := c.Param("version")
		filename := c.Param("filename")

		if packageID == "" || version == "" || filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID, version, and filename required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		
		artifact, content, err := registryService.Download(ctx, packageID, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}

		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		
		if artifact.Size > 0 {
			c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream package content"})
			return
		}
	}
}

func handleNuGetUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		file, header, err := c.Request.FormFile("package")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package file required"})
			return
		}
		defer file.Close()

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err = registryService.Upload(ctx, "nuget", header.Filename, file, "application/zip")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "package uploaded successfully",
		})
	}
}

func handleNuGetDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		packageID := c.Param("id")
		version := c.Param("version")

		if packageID == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, packageID, version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete package"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "package deleted successfully",
		})
	}
}

func handleNuGetSearch(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		skip := c.DefaultQuery("skip", "0")
		take := c.DefaultQuery("take", "20")

		skipInt, _ := strconv.Atoi(skip)
		takeInt, _ := strconv.Atoi(take)

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		
		filter := &types.ArtifactFilter{
			Registry: "nuget",
			Limit:    takeInt,
			Offset:   skipInt,
		}

		if query != "" {
			filter.Name = query // Simple name-based search for now
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		// Convert to NuGet search response format
		results := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			var description, author string
			if artifact.Metadata != nil {
				if desc, ok := artifact.Metadata["description"].(string); ok {
					description = desc
				}
				if auth, ok := artifact.Metadata["author"].(string); ok {
					author = auth
				}
			}

			results = append(results, gin.H{
				"id":          artifact.Name,
				"version":     artifact.Version,
				"description": description,
				"authors":     []string{author},
				"totalDownloads": 0, // TODO: implement download counting
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"totalHits": len(results),
			"data":      results,
		})
	}
}

func handleNuGetPackageMetadata(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		if packageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		
		filter := &types.ArtifactFilter{
			Name:     packageID,
			Registry: "nuget",
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get package metadata"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}

		// Build NuGet registration response
		items := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			var description string
			if artifact.Metadata != nil {
				if desc, ok := artifact.Metadata["description"].(string); ok {
					description = desc
				}
			}

			items = append(items, gin.H{
				"catalogEntry": gin.H{
					"id":          artifact.Name,
					"version":     artifact.Version,
					"description": description,
					"published":   artifact.CreatedAt,
				},
				"packageContent": fmt.Sprintf("/nuget/v3-flatcontainer/%s/%s/%s.%s.nupkg", 
					strings.ToLower(artifact.Name), artifact.Version, 
					strings.ToLower(artifact.Name), artifact.Version),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"count": len(items),
			"items": []gin.H{
				{
					"items": items,
				},
			},
		})
	}
}
