package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// RubyGemsRoutes sets up RubyGems repository routes
func RubyGemsRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	gems := api.Group("/gems")
	
	// RubyGems API
	gems.GET("/api/v1/gems", handleGemsSearch(registryService))
	gems.GET("/api/v1/gems/:name.json", handleGemInfo(registryService))
	gems.GET("/api/v1/versions/:name.json", handleGemVersions(registryService))
	gems.GET("/gems/:filename", handleGemDownload(registryService))
	
	// Gem push (requires authentication)
	gems.POST("/api/v1/gems", middleware.AuthMiddleware(authService), handleGemPush(registryService))
	gems.DELETE("/api/v1/gems/yank", middleware.AuthMiddleware(authService), handleGemYank(registryService))
	
	// Specs endpoints for bundler
	gems.GET("/specs.4.8.gz", handleSpecs(registryService))
	gems.GET("/latest_specs.4.8.gz", handleLatestSpecs(registryService))
	gems.GET("/prerelease_specs.4.8.gz", handlePrereleaseSpecs(registryService))
}

func handleGemsSearch(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("query")
		
		ctx := context.WithValue(c.Request.Context(), "registry", "rubygems")
		
		filter := &types.ArtifactFilter{
			Registry: "rubygems",
		}
		
		if query != "" {
			filter.Name = query
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		// Convert to RubyGems search response format
		gems := make([]gin.H, 0, len(artifacts))
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

			gems = append(gems, gin.H{
				"name":        artifact.Name,
				"version":     artifact.Version,
				"description": description,
				"authors":     author,
				"info":        fmt.Sprintf("https://rubygems.org/gems/%s", artifact.Name),
			})
		}

		c.JSON(http.StatusOK, gems)
	}
}

func handleGemInfo(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		gemName := c.Param("name")
		if gemName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "gem name required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "rubygems")
		
		filter := &types.ArtifactFilter{
			Name:     gemName,
			Registry: "rubygems",
			Limit:    1, // Get latest version
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get gem info"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "gem not found"})
			return
		}

		artifact := artifacts[0]
		var description, author string
		if artifact.Metadata != nil {
			if desc, ok := artifact.Metadata["description"].(string); ok {
				description = desc
			}
			if auth, ok := artifact.Metadata["author"].(string); ok {
				author = auth
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"name":        artifact.Name,
			"version":     artifact.Version,
			"description": description,
			"authors":     author,
			"gem_uri":     fmt.Sprintf("/gems/%s-%s.gem", artifact.Name, artifact.Version),
			"homepage_uri": "",
			"source_code_uri": "",
		})
	}
}

func handleGemVersions(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		gemName := c.Param("name")
		if gemName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "gem name required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "rubygems")
		
		filter := &types.ArtifactFilter{
			Name:     gemName,
			Registry: "rubygems",
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get gem versions"})
			return
		}

		versions := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			versions = append(versions, gin.H{
				"number":     artifact.Version,
				"created_at": artifact.CreatedAt,
				"prerelease": false, // TODO: implement prerelease detection
			})
		}

		c.JSON(http.StatusOK, versions)
	}
}

func handleGemDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")
		if filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "filename required"})
			return
		}

		// Parse gem filename: gemname-version.gem
		// This is a simplified parser
		if len(filename) < 5 || !strings.HasSuffix(filename, ".gem") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gem filename"})
			return
		}

		nameVersion := strings.TrimSuffix(filename, ".gem")
		parts := strings.Split(nameVersion, "-")
		if len(parts) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gem filename format"})
			return
		}

		version := parts[len(parts)-1]
		gemName := strings.Join(parts[:len(parts)-1], "-")

		ctx := context.WithValue(c.Request.Context(), "registry", "rubygems")
		
		artifact, content, err := registryService.Download(ctx, gemName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "gem not found"})
			return
		}

		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		
		if artifact.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream gem"})
			return
		}
	}
}

func handleGemPush(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		file, header, err := c.Request.FormFile("gem")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "gem file required"})
			return
		}
		defer file.Close()

		ctx := context.WithValue(c.Request.Context(), "registry", "rubygems")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err = registryService.Upload(ctx, "rubygems", header.Filename, file, "application/octet-stream")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.String(http.StatusOK, "Successfully registered gem")
	}
}

func handleGemYank(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		gemName := c.Query("gem_name")
		version := c.Query("version")

		if gemName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "gem_name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "rubygems")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, gemName, version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to yank gem"})
			return
		}

		c.String(http.StatusOK, "Successfully yanked gem")
	}
}

// Specs endpoints - simplified implementations
func handleSpecs(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, this would return a gzipped marshaled Ruby array
		// of all gem specifications
		c.Header("Content-Type", "application/octet-stream")
		c.String(http.StatusOK, "")
	}
}

func handleLatestSpecs(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Latest versions only
		c.Header("Content-Type", "application/octet-stream")
		c.String(http.StatusOK, "")
	}
}

func handlePrereleaseSpecs(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prerelease versions only
		c.Header("Content-Type", "application/octet-stream")
		c.String(http.StatusOK, "")
	}
}
