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

// GoRoutes sets up Go module proxy routes
func GoRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	goproxy := api.Group("/go")

	// Go module proxy protocol
	goproxy.GET("/:module/@latest", handleGoLatest(registryService))
	goproxy.GET("/:module/@v/list", handleGoVersionList(registryService))
	
	// Handle versioned operations - this will detect file extensions like .info, .mod, .zip
	goproxy.GET("/:module/@v/:version", handleGoVersionFile(registryService))
	
	// Module upload and delete (custom extension to Go proxy protocol)
	goproxy.PUT("/:module/@v/:version", middleware.AuthMiddleware(authService), handleGoModuleUpload(registryService))
	goproxy.DELETE("/:module/@v/:version", middleware.AuthMiddleware(authService), handleGoModuleDelete(registryService))
}

func handleGoVersionList(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		module := c.Param("module")
		if module == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "module path required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "go")

		filter := &types.ArtifactFilter{
			Name:     module,
			Registry: "go",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
			return
		}

		versions := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			versions = append(versions, artifact.Version)
		}

		// Return versions as plain text, one per line
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, strings.Join(versions, "\n"))
	}
}

func handleGoVersionFile(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		module := c.Param("module")
		versionParam := c.Param("version")

		if module == "" || versionParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "module path and version required"})
			return
		}

		// Parse version and file type from the parameter (e.g., "v1.0.0.info", "v1.0.0.mod", "v1.0.0.zip")
		parts := strings.Split(versionParam, ".")
		if len(parts) < 2 {
			// If no file extension, treat as regular version download (.zip)
			version := versionParam
			
			// Handle .zip download
			artifact, content, err := registryService.Download(c.Request.Context(), "go", module, version)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "module not found"})
				return
			}

			c.Header("Content-Type", "application/zip")
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s@%s.zip", module, version))

			if artifact.Size > 0 {
				c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
			}

			defer content.Close()
			_, err = io.Copy(c.Writer, content)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream module"})
				return
			}
			return
		}

		version := strings.Join(parts[:len(parts)-1], ".") // Everything except the last part
		fileType := parts[len(parts)-1]                    // The file extension

		ctx := context.WithValue(c.Request.Context(), "registry", "go")

		switch fileType {
		case "info":
			// Go module info format
			artifact, _, err := registryService.Download(ctx, "go", module, version)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
				return
			}

			info := fmt.Sprintf(`{
	"Version": "%s",
	"Time": "%s"
}`, artifact.Version, artifact.CreatedAt.Format("2006-01-02T15:04:05Z"))

			c.Header("Content-Type", "application/json")
			c.String(http.StatusOK, info)

		case "mod":
			// For now, return a simple go.mod file
			// In a real implementation, this would be extracted from the uploaded module
			modContent := fmt.Sprintf("module %s\n\ngo 1.19\n", module)
			c.Header("Content-Type", "text/plain")
			c.String(http.StatusOK, modContent)

		case "zip":
			artifact, content, err := registryService.Download(ctx, "go", module, version)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "module not found"})
				return
			}

			c.Header("Content-Type", "application/zip")
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s@%s.zip", module, version))

			if artifact.Size > 0 {
				c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
			}

			defer content.Close()
			_, err = io.Copy(c.Writer, content)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream module"})
				return
			}

		default:
			c.JSON(http.StatusNotFound, gin.H{"error": "unsupported file type"})
		}
	}
}

func handleGoLatest(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		module := c.Param("module")
		if module == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "module path required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "go")

		filter := &types.ArtifactFilter{
			Name:     module,
			Registry: "go",
			Limit:    1, // Get the latest version
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get latest version"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "module not found"})
			return
		}

		latest := artifacts[0]
		info := fmt.Sprintf(`{
	"Version": "%s",
	"Time": "%s"
}`, latest.Version, latest.CreatedAt.Format("2006-01-02T15:04:05Z"))

		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, info)
	}
}

func handleGoModuleUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		module := c.Param("module")
		version := c.Param("version")

		if module == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "module path and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "go")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		_, err := registryService.Upload(ctx, "go", module, version, c.Request.Body, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "module uploaded successfully",
		})
	}
}

func handleGoModuleDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		module := c.Param("module")
		version := c.Param("version")

		if module == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "module path and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "go")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, "go", module, version, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete module"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "module deleted successfully",
		})
	}
}
