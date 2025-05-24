package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// CargoRoutes sets up Rust Cargo registry routes
func CargoRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	cargo := api.Group("/cargo")
	
	// Cargo registry API
	cargo.GET("/api/v1/crates", handleCargoSearch(registryService))
	cargo.GET("/api/v1/crates/:crate", handleCargoInfo(registryService))
	cargo.GET("/api/v1/crates/:crate/:version/download", handleCargoDownload(registryService))
	
	// Crate publish (requires authentication)
	cargo.PUT("/api/v1/crates/new", middleware.AuthMiddleware(authService), handleCargoPublish(registryService))
	cargo.DELETE("/api/v1/crates/:crate/:version/yank", middleware.AuthMiddleware(authService), handleCargoYank(registryService))
}

func handleCargoSearch(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		
		ctx := context.WithValue(c.Request.Context(), "registry", "cargo")
		
		filter := &types.ArtifactFilter{
			Registry: "cargo",
		}
		
		if query != "" {
			filter.Name = query
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		// Convert to Cargo search response format
		crates := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			var description string
			if artifact.Metadata != nil {
				if desc, ok := artifact.Metadata["description"].(string); ok {
					description = desc
				}
			}

			crates = append(crates, gin.H{
				"name":        artifact.Name,
				"max_version": artifact.Version,
				"description": description,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"crates": crates,
			"meta": gin.H{
				"total": len(crates),
			},
		})
	}
}

func handleCargoInfo(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		crateName := c.Param("crate")
		if crateName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "crate name required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "cargo")
		
		filter := &types.ArtifactFilter{
			Name:     crateName,
			Registry: "cargo",
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get crate info"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "crate not found"})
			return
		}

		// Build versions array
		versions := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			versions = append(versions, gin.H{
				"num":        artifact.Version,
				"dl_path":    fmt.Sprintf("/cargo/api/v1/crates/%s/%s/download", crateName, artifact.Version),
				"created_at": artifact.CreatedAt,
				"yanked":     false, // TODO: implement yanking
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"crate": gin.H{
				"name": crateName,
			},
			"versions": versions,
		})
	}
}

func handleCargoDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		crateName := c.Param("crate")
		version := c.Param("version")

		if crateName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "crate name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "cargo")
		
		artifact, content, err := registryService.Download(ctx, crateName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "crate not found"})
			return
		}

		c.Header("Content-Type", "application/gzip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.crate", crateName, version))
		
		if artifact.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream crate"})
			return
		}
	}
}

func handleCargoPublish(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Cargo publish sends the crate as multipart form data
		file, header, err := c.Request.FormFile("crate")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "crate file required"})
			return
		}
		defer file.Close()

		ctx := context.WithValue(c.Request.Context(), "registry", "cargo")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err = registryService.Upload(ctx, "cargo", header.Filename, file, "application/gzip")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"warnings": gin.H{
				"invalid_categories": []string{},
				"invalid_badges":     []string{},
				"other":              []string{},
			},
		})
	}
}

func handleCargoYank(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		crateName := c.Param("crate")
		version := c.Param("version")

		if crateName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "crate name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "cargo")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// For now, yanking is equivalent to deletion
		// In a real implementation, you'd mark the version as yanked instead
		err := registryService.Delete(ctx, crateName, version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to yank crate"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
		})
	}
}
