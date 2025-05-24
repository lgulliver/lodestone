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

// HelmRoutes sets up Helm chart repository routes
func HelmRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	helm := api.Group("/helm")

	// Helm repository API
	helm.GET("/index.yaml", handleHelmIndex(registryService))
	helm.GET("/:chart/:version/:filename", middleware.OptionalAuthMiddleware(authService), handleHelmDownload(registryService))

	// Chart upload (requires authentication)
	helm.POST("/api/charts", middleware.AuthMiddleware(authService), handleHelmUpload(registryService))
	helm.DELETE("/api/charts/:chart/:version", middleware.AuthMiddleware(authService), handleHelmDelete(registryService))
}

func handleHelmIndex(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "registry", "helm")

		filter := &types.ArtifactFilter{
			Registry: "helm",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate index"})
			return
		}

		// Build Helm index.yaml
		entries := make(map[string][]map[string]interface{})

		for _, artifact := range artifacts {
			chartName := artifact.Name

			var description, maintainer string
			if artifact.Metadata != nil {
				if desc, ok := artifact.Metadata["description"].(string); ok {
					description = desc
				}
				if maint, ok := artifact.Metadata["maintainer"].(string); ok {
					maintainer = maint
				}
			}

			chartEntry := map[string]interface{}{
				"name":        chartName,
				"version":     artifact.Version,
				"description": description,
				"created":     artifact.CreatedAt.Format("2006-01-02T15:04:05.000000000Z"),
				"digest":      artifact.SHA256,
				"urls": []string{
					fmt.Sprintf("/helm/%s/%s/%s-%s.tgz", chartName, artifact.Version, chartName, artifact.Version),
				},
			}

			if maintainer != "" {
				chartEntry["maintainers"] = []map[string]string{
					{"name": maintainer},
				}
			}

			if entries[chartName] == nil {
				entries[chartName] = make([]map[string]interface{}, 0)
			}
			entries[chartName] = append(entries[chartName], chartEntry)
		}

		index := map[string]interface{}{
			"apiVersion": "v1",
			"entries":    entries,
			"generated":  "2024-01-01T00:00:00Z", // TODO: Use current time
		}

		c.Header("Content-Type", "application/x-yaml")
		c.JSON(http.StatusOK, index)
	}
}

func handleHelmDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		chart := c.Param("chart")
		version := c.Param("version")
		filename := c.Param("filename")

		if chart == "" || version == "" || filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chart name, version, and filename required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "helm")

		artifact, content, err := registryService.Download(ctx, "helm", chart, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "chart not found"})
			return
		}

		c.Header("Content-Type", "application/gzip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		if artifact.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream chart"})
			return
		}
	}
}

func handleHelmUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		file, header, err := c.Request.FormFile("chart")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chart file required"})
			return
		}
		defer file.Close()

		ctx := context.WithValue(c.Request.Context(), "registry", "helm")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// Parse Helm chart filename: chartname-version.tgz
		filename := header.Filename
		if !strings.HasSuffix(filename, ".tgz") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chart file must be .tgz format"})
			return
		}

		nameVersion := strings.TrimSuffix(filename, ".tgz")
		parts := strings.Split(nameVersion, "-")
		if len(parts) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chart filename format, expected: chartname-version.tgz"})
			return
		}

		// Last part is version, everything else is chart name
		version := parts[len(parts)-1]
		chartName := strings.Join(parts[:len(parts)-1], "-")

		_, err = registryService.Upload(ctx, "helm", chartName, version, file, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "chart uploaded successfully",
		})
	}
}

func handleHelmDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		chart := c.Param("chart")
		version := c.Param("version")

		if chart == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chart name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "helm")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, "helm", chart, version, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete chart"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "chart deleted successfully",
		})
	}
}
