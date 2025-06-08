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
)

// MavenRoutes sets up Maven repository routes
func MavenRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	maven := api.Group("/maven")

	// Maven repository structure: groupId/artifactId/version/artifactId-version.jar - requires authentication
	maven.GET("/*path", middleware.AuthMiddleware(authService), handleMavenDownload(registryService))
	maven.PUT("/*path", middleware.AuthMiddleware(authService), handleMavenUpload(registryService))
	maven.HEAD("/*path", middleware.AuthMiddleware(authService), handleMavenHead(registryService))
	maven.DELETE("/*path", middleware.AuthMiddleware(authService), handleMavenDelete(registryService))
}

func handleMavenDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("path"), "/")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}

		// Parse Maven path: groupId/artifactId/version/filename
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Maven path"})
			return
		}

		groupId := strings.Join(parts[:len(parts)-3], ".")
		artifactId := parts[len(parts)-3]
		version := parts[len(parts)-2]
		filename := parts[len(parts)-1]

		packageName := fmt.Sprintf("%s:%s", groupId, artifactId)

		ctx := context.WithValue(c.Request.Context(), "registry", "maven")

		artifact, content, err := registryService.Download(ctx, "maven", packageName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
			return
		}

		c.Header("Content-Type", "application/java-archive")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		if artifact.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream artifact"})
			return
		}
	}
}

func handleMavenUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		path := strings.TrimPrefix(c.Param("path"), "/")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "maven")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		contentType := c.GetHeader("Content-Type")
		if contentType == "" {
			if strings.HasSuffix(path, ".jar") {
				contentType = "application/java-archive"
			} else if strings.HasSuffix(path, ".pom") {
				contentType = "application/xml"
			} else {
				contentType = "application/octet-stream"
			}
		}

		// Parse Maven path to extract groupId, artifactId, and version
		// Format: com/example/artifact/1.0.0/artifact-1.0.0.jar
		pathParts := strings.Split(path, "/")
		if len(pathParts) < 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Maven path format"})
			return
		}

		// Extract version and filename
		version := pathParts[len(pathParts)-2]
		filename := pathParts[len(pathParts)-1]

		// Extract artifact ID from filename (remove version and extension)
		artifactID := strings.Split(filename, "-")[0]

		// Construct full artifact name (groupId:artifactId)
		groupId := strings.Join(pathParts[:len(pathParts)-2], ".")
		fullName := fmt.Sprintf("%s:%s", groupId, artifactID)

		_, err := registryService.Upload(ctx, "maven", fullName, version, c.Request.Body, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.Status(http.StatusCreated)
	}
}

func handleMavenHead(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("path"), "/")
		if path == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		// Parse Maven path to extract package name and version
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			c.Status(http.StatusBadRequest)
			return
		}

		groupId := strings.Join(parts[:len(parts)-3], ".")
		artifactId := parts[len(parts)-3]
		version := parts[len(parts)-2]
		packageName := fmt.Sprintf("%s:%s", groupId, artifactId)

		ctx := context.WithValue(c.Request.Context(), "registry", "maven")

		artifact, _, err := registryService.Download(ctx, "maven", packageName, version)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", "application/java-archive")
		c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		c.Status(http.StatusOK)
	}
}

func handleMavenDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		path := strings.TrimPrefix(c.Param("path"), "/")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}

		// Parse Maven path
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Maven path"})
			return
		}

		groupId := strings.Join(parts[:len(parts)-3], ".")
		artifactId := parts[len(parts)-3]
		version := parts[len(parts)-2]
		packageName := fmt.Sprintf("%s:%s", groupId, artifactId)

		ctx := context.WithValue(c.Request.Context(), "registry", "maven")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, "maven", packageName, version, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete artifact"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
