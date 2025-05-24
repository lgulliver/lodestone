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

// OCIRoutes sets up OCI (Docker) registry routes
func OCIRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	oci := api.Group("/v2")
	
	// Base endpoint for Docker registry API compatibility
	oci.GET("/", handleOCIBase())
	
	// Image manifest operations
	oci.GET("/:name/manifests/:reference", middleware.OptionalAuthMiddleware(authService), handleOCIManifestGet(registryService))
	oci.PUT("/:name/manifests/:reference", middleware.AuthMiddleware(authService), handleOCIManifestPut(registryService))
	oci.DELETE("/:name/manifests/:reference", middleware.AuthMiddleware(authService), handleOCIManifestDelete(registryService))
	oci.HEAD("/:name/manifests/:reference", middleware.OptionalAuthMiddleware(authService), handleOCIManifestHead(registryService))
	
	// Blob operations
	oci.GET("/:name/blobs/:digest", middleware.OptionalAuthMiddleware(authService), handleOCIBlobGet(registryService))
	oci.HEAD("/:name/blobs/:digest", middleware.OptionalAuthMiddleware(authService), handleOCIBlobHead(registryService))
	oci.DELETE("/:name/blobs/:digest", middleware.AuthMiddleware(authService), handleOCIBlobDelete(registryService))
	
	// Blob upload operations
	oci.POST("/:name/blobs/uploads/", middleware.AuthMiddleware(authService), handleOCIBlobUploadStart(registryService))
	oci.PATCH("/:name/blobs/uploads/:uuid", middleware.AuthMiddleware(authService), handleOCIBlobUploadChunk(registryService))
	oci.PUT("/:name/blobs/uploads/:uuid", middleware.AuthMiddleware(authService), handleOCIBlobUploadComplete(registryService))
	oci.DELETE("/:name/blobs/uploads/:uuid", middleware.AuthMiddleware(authService), handleOCIBlobUploadCancel(registryService))
	oci.GET("/:name/blobs/uploads/:uuid", middleware.AuthMiddleware(authService), handleOCIBlobUploadStatus(registryService))
	
	// Tag listing
	oci.GET("/:name/tags/list", middleware.OptionalAuthMiddleware(authService), handleOCITagsList(registryService))
	
	// Catalog (repository listing)
	oci.GET("/_catalog", middleware.OptionalAuthMiddleware(authService), handleOCICatalog(registryService))
}

func handleOCIBase() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.JSON(http.StatusOK, gin.H{
			"name": "Lodestone OCI Registry",
		})
	}
}

func handleOCIManifestGet(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")

		if name == "" || reference == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repository name and reference required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		
		// For OCI, the reference could be a tag or digest
		var version string
		if strings.HasPrefix(reference, "sha256:") {
			// It's a digest, need to look up by SHA256
			filter := &types.ArtifactFilter{
				Name:     name,
				Registry: "oci",
			}
			
			artifacts, err := registryService.List(ctx, filter)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list manifests"})
				return
			}
			
			for _, artifact := range artifacts {
				if artifact.SHA256 == reference {
					version = artifact.Version
					break
				}
			}
			
			if version == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
				return
			}
		} else {
			version = reference
		}

		artifact, content, err := registryService.Download(ctx, name, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
			return
		}

		c.Header("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		c.Header("Docker-Content-Digest", fmt.Sprintf("sha256:%s", artifact.SHA256))
		
		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream manifest"})
			return
		}
	}
}

func handleOCIManifestPut(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := c.Param("name")
		reference := c.Param("reference")

		if name == "" || reference == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repository name and reference required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		contentType := c.GetHeader("Content-Type")
		if contentType == "" {
			contentType = "application/vnd.docker.distribution.manifest.v2+json"
		}

		err := registryService.Upload(ctx, "oci", fmt.Sprintf("%s:%s", name, reference), c.Request.Body, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.Header("Location", fmt.Sprintf("/v2/%s/manifests/%s", name, reference))
		c.Status(http.StatusCreated)
	}
}

func handleOCIManifestDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := c.Param("name")
		reference := c.Param("reference")

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, name, reference)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete manifest"})
			return
		}

		c.Status(http.StatusAccepted)
	}
}

func handleOCIManifestHead(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		
		artifact, _, err := registryService.Download(ctx, name, reference)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		c.Header("Docker-Content-Digest", fmt.Sprintf("sha256:%s", artifact.SHA256))
		c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
		c.Status(http.StatusOK)
	}
}

func handleOCIBlobGet(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		digest := c.Param("digest")

		if !strings.HasPrefix(digest, "sha256:") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid digest format"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		
		// Find artifact by SHA256
		filter := &types.ArtifactFilter{
			Name:     name,
			Registry: "oci",
		}
		
		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list blobs"})
			return
		}
		
		var targetArtifact *types.Artifact
		for _, artifact := range artifacts {
			if fmt.Sprintf("sha256:%s", artifact.SHA256) == digest {
				targetArtifact = artifact
				break
			}
		}
		
		if targetArtifact == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "blob not found"})
			return
		}

		_, content, err := registryService.Download(ctx, name, targetArtifact.Version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "blob not found"})
			return
		}

		c.Header("Content-Type", "application/octet-stream")
		c.Header("Docker-Content-Digest", digest)
		
		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream blob"})
			return
		}
	}
}

func handleOCIBlobHead(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		digest := c.Param("digest")

		if !strings.HasPrefix(digest, "sha256:") {
			c.Status(http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		
		filter := &types.ArtifactFilter{
			Name:     name,
			Registry: "oci",
		}
		
		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		
		for _, artifact := range artifacts {
			if fmt.Sprintf("sha256:%s", artifact.SHA256) == digest {
				c.Header("Content-Type", "application/octet-stream")
				c.Header("Docker-Content-Digest", digest)
				c.Header("Content-Length", fmt.Sprintf("%d", artifact.Size))
				c.Status(http.StatusOK)
				return
			}
		}
		
		c.Status(http.StatusNotFound)
	}
}

func handleOCIBlobDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := c.Param("name")
		digest := c.Param("digest")

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// Find and delete by digest
		filter := &types.ArtifactFilter{
			Name:     name,
			Registry: "oci",
		}
		
		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list blobs"})
			return
		}
		
		for _, artifact := range artifacts {
			if fmt.Sprintf("sha256:%s", artifact.SHA256) == digest {
				err := registryService.Delete(ctx, name, artifact.Version)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete blob"})
					return
				}
				c.Status(http.StatusAccepted)
				return
			}
		}
		
		c.Status(http.StatusNotFound)
	}
}

// Blob upload handlers - simplified implementations
func handleOCIBlobUploadStart(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := c.Param("name")
		
		// Generate a UUID for the upload session
		uploadUUID := fmt.Sprintf("upload-%s-%d", name, user.ID)
		
		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, uploadUUID))
		c.Header("Range", "0-0")
		c.Status(http.StatusAccepted)
	}
}

func handleOCIBlobUploadChunk(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simplified - in real implementation, would handle chunked uploads
		c.Status(http.StatusAccepted)
	}
}

func handleOCIBlobUploadComplete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := c.Param("name")
		uuid := c.Param("uuid")
		digest := c.Query("digest")

		if digest == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "digest parameter required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Upload(ctx, "oci", fmt.Sprintf("%s:%s", name, uuid), c.Request.Body, "application/octet-stream")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
		c.Status(http.StatusCreated)
	}
}

func handleOCIBlobUploadCancel(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	}
}

func handleOCIBlobUploadStatus(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Range", "0-0")
		c.Status(http.StatusNoContent)
	}
}

func handleOCITagsList(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		
		filter := &types.ArtifactFilter{
			Name:     name,
			Registry: "oci",
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tags"})
			return
		}

		tags := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			tags = append(tags, artifact.Version)
		}

		c.JSON(http.StatusOK, gin.H{
			"name": name,
			"tags": tags,
		})
	}
}

func handleOCICatalog(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		
		filter := &types.ArtifactFilter{
			Registry: "oci",
		}

		artifacts, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list repositories"})
			return
		}

		// Extract unique repository names
		repoSet := make(map[string]bool)
		for _, artifact := range artifacts {
			repoSet[artifact.Name] = true
		}

		repositories := make([]string, 0, len(repoSet))
		for repo := range repoSet {
			repositories = append(repositories, repo)
		}

		c.JSON(http.StatusOK, gin.H{
			"repositories": repositories,
		})
	}
}
