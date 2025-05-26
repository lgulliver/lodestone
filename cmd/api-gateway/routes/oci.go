package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/registry/registries/oci"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
)

// OCIRoutes sets up OCI (Docker) registry routes
func OCIRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	oci := api.Group("/v2")

	// Docker authentication endpoints
	oci.GET("/auth", handleDockerAuth(authService))
	oci.POST("/auth", handleDockerAuth(authService))
	oci.GET("/token", handleDockerToken(authService))
	oci.POST("/token", handleDockerToken(authService))

	// Note: Base endpoint (/v2/) is handled by OCIRootRoutes catch-all handler

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

// OCIRootRoutes sets up OCI (Docker) registry routes at root level for Docker CLI compatibility
func OCIRootRoutes(router *gin.Engine, registryService *registry.Service, authService *auth.Service) {
	// Use a catch-all route for all OCI operations including the base endpoint
	router.Any("/v2/*path", handleOCIRequest(registryService, authService))
}

// Helper function to extract repository name from wildcard parameter
// Gin wildcard parameters include the leading slash, so we need to strip it
func extractRepositoryName(c *gin.Context) string {
	name := c.Param("name")
	if strings.HasPrefix(name, "/") {
		return name[1:] // Remove leading slash
	}
	return name
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
		name := extractRepositoryName(c)
		reference := c.Param("reference")

		if name == "" || reference == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repository name and reference required"})
			return
		}

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Get manifest using enhanced method
		manifest, digest, size, err := ociRegistry.GetManifest(c.Request.Context(), name, reference)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve manifest"})
			}
			return
		}
		defer manifest.Close()

		// Read the manifest content to detect media type
		manifestContent, err := io.ReadAll(manifest)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read manifest content"})
			return
		}

		// Parse manifest to detect media type
		var manifestObj map[string]interface{}
		if err := json.Unmarshal(manifestContent, &manifestObj); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid manifest format"})
			return
		}

		// Determine content type based on manifest mediaType
		contentType := "application/vnd.docker.distribution.manifest.v2+json" // default
		if mediaType, ok := manifestObj["mediaType"].(string); ok {
			contentType = mediaType
		}

		c.Header("Content-Type", contentType)
		c.Header("Docker-Content-Digest", digest)
		c.Header("Content-Length", fmt.Sprintf("%d", size))

		c.Data(http.StatusOK, contentType, manifestContent)
		if err != nil {
			log.Error().Err(err).Str("repository", name).Str("reference", reference).Msg("Failed to stream manifest")
			return
		}

		log.Debug().
			Str("repository", name).
			Str("reference", reference).
			Str("digest", digest).
			Msg("Successfully served manifest")
	}
}

func handleOCIManifestPut(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := extractRepositoryName(c)
		reference := c.Param("reference")

		if name == "" || reference == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repository name and reference required"})
			return
		}

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		contentType := c.GetHeader("Content-Type")
		if contentType == "" {
			contentType = "application/vnd.docker.distribution.manifest.v2+json"
		}

		// Store manifest using enhanced method
		digest, err := ociRegistry.PutManifest(c.Request.Context(), name, reference, c.Request.Body, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to store manifest: %v", err)})
			return
		}

		// Create artifact record in database
		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		_, err = registryService.Upload(ctx, "oci", name, reference, strings.NewReader(""), user.ID)
		if err != nil {
			log.Warn().Err(err).Str("repository", name).Str("reference", reference).Msg("Failed to create artifact record")
		}

		c.Header("Location", fmt.Sprintf("/v2/%s/manifests/%s", name, reference))
		c.Header("Docker-Content-Digest", digest)
		c.Status(http.StatusCreated)

		log.Info().
			Str("repository", name).
			Str("reference", reference).
			Str("digest", digest).
			Str("user_id", user.ID.String()).
			Msg("Successfully stored manifest")
	}
}

func handleOCIManifestDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, userExists := middleware.GetUserFromContext(c)
		if !userExists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := c.Param("name")
		reference := c.Param("reference")

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Check if manifest exists first
		manifestExists, _, _, _, err := ociRegistry.ManifestExists(c.Request.Context(), name, reference)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check manifest existence"})
			return
		}

		if !manifestExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
			return
		}

		// Delete manifest using enhanced method
		err = ociRegistry.DeleteManifest(c.Request.Context(), name, reference)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete manifest"})
			return
		}

		// Also delete from database
		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err = registryService.Delete(ctx, "oci", name, reference, user.ID)
		if err != nil {
			log.Warn().Err(err).Str("repository", name).Str("reference", reference).Msg("Failed to delete artifact record from database")
		}

		c.Status(http.StatusAccepted)

		log.Info().
			Str("repository", name).
			Str("reference", reference).
			Str("user_id", user.ID.String()).
			Msg("Successfully deleted manifest")
	}
}

func handleOCIManifestHead(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}

		// Check if manifest exists
		exists, digest, size, mediaType, err := ociRegistry.ManifestExists(c.Request.Context(), name, reference)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		if !exists {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", mediaType)
		c.Header("Docker-Content-Digest", digest)
		c.Header("Content-Length", fmt.Sprintf("%d", size))
		c.Status(http.StatusOK)

		log.Debug().
			Str("repository", name).
			Str("reference", reference).
			Str("digest", digest).
			Int64("size", size).
			Msg("Successfully checked manifest existence")
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

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Get blob from storage
		reader, size, err := ociRegistry.GetBlob(c.Request.Context(), name, digest)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "blob not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve blob"})
			}
			return
		}
		defer reader.Close()

		c.Header("Content-Type", "application/octet-stream")
		c.Header("Docker-Content-Digest", digest)
		c.Header("Content-Length", fmt.Sprintf("%d", size))

		_, err = io.Copy(c.Writer, reader)
		if err != nil {
			log.Error().Err(err).Str("digest", digest).Msg("Failed to stream blob")
			return
		}

		log.Debug().
			Str("repository", name).
			Str("digest", digest).
			Int64("size", size).
			Msg("Successfully served blob")
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

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}

		// Check if blob exists
		exists, size, err := ociRegistry.BlobExists(c.Request.Context(), name, digest)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		if !exists {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", "application/octet-stream")
		c.Header("Docker-Content-Digest", digest)
		c.Header("Content-Length", fmt.Sprintf("%d", size))
		c.Status(http.StatusOK)

		log.Debug().
			Str("repository", name).
			Str("digest", digest).
			Int64("size", size).
			Msg("Successfully checked blob existence")
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

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list blobs"})
			return
		}

		for _, artifact := range artifacts {
			if fmt.Sprintf("sha256:%s", artifact.SHA256) == digest {
				err := registryService.Delete(ctx, "oci", name, artifact.Version, user.ID)
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

// Blob upload handlers - enhanced implementations with session management
func handleOCIBlobUploadStart(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := extractRepositoryName(c)

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Start upload session
		session, err := ociRegistry.StartBlobUpload(c.Request.Context(), name, user.ID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to start upload: %v", err)})
			return
		}

		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, session.ID))
		c.Header("Range", "0-0")
		c.Header("Docker-Upload-UUID", session.ID)
		c.Status(http.StatusAccepted)

		log.Info().
			Str("session_id", session.ID).
			Str("repository", name).
			Str("user_id", user.ID.String()).
			Msg("Started blob upload session")
	}
}

func handleOCIBlobUploadChunk(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		sessionID := c.Param("uuid")
		contentRange := c.GetHeader("Content-Range")

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Append chunk to session
		session, err := ociRegistry.AppendBlobChunk(c.Request.Context(), sessionID, c.Request.Body, contentRange)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("upload session error: %v", err)})
			return
		}

		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", session.Repository, sessionID))
		c.Header("Range", fmt.Sprintf("0-%d", session.Size-1))
		c.Header("Docker-Upload-UUID", sessionID)
		c.Status(http.StatusAccepted)

		log.Debug().
			Str("session_id", sessionID).
			Int64("chunk_size", session.Size).
			Msg("Appended chunk to blob upload")
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
		sessionID := c.Param("uuid")
		digest := c.Query("digest")

		if digest == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "digest parameter required"})
			return
		}

		if !strings.HasPrefix(digest, "sha256:") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid digest format, must start with sha256:"})
			return
		}

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Handle any final chunk data in the request body
		if c.Request.ContentLength > 0 {
			_, err := ociRegistry.AppendBlobChunk(c.Request.Context(), sessionID, c.Request.Body, "")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to append final chunk: %v", err)})
				return
			}
		}

		// Complete the upload with digest verification
		session, storagePath, err := ociRegistry.CompleteBlobUpload(c.Request.Context(), sessionID, digest)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upload completion failed: %v", err)})
			return
		}

		// Create artifact record in database
		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		artifact := &types.Artifact{
			Name:        name,
			Version:     digest, // For blobs, version is the digest
			Registry:    "oci",
			Size:        session.Size,
			SHA256:      strings.TrimPrefix(digest, "sha256:"),
			StoragePath: storagePath,
			PublishedBy: user.ID,
			IsPublic:    false,
			ContentType: "application/octet-stream",
		}

		// Save to database
		if err := registryService.DB.Create(artifact).Error; err != nil {
			log.Error().Err(err).Str("digest", digest).Msg("Failed to save blob artifact to database")
			// Don't return error as the blob is already stored successfully
		}

		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
		c.Header("Docker-Content-Digest", digest)
		c.Status(http.StatusCreated)

		log.Info().
			Str("session_id", sessionID).
			Str("repository", name).
			Str("digest", digest).
			Int64("size", session.Size).
			Str("user_id", user.ID.String()).
			Msg("Completed blob upload")
	}
}

func handleOCIBlobUploadCancel(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		sessionID := c.Param("uuid")

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Cancel the upload session
		err = ociRegistry.CancelBlobUpload(c.Request.Context(), sessionID)
		if err != nil {
			log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to cancel upload session")
			// Don't return error as the session might not exist
		}

		c.Status(http.StatusNoContent)

		log.Info().
			Str("session_id", sessionID).
			Msg("Cancelled blob upload session")
	}
}

func handleOCIBlobUploadStatus(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		sessionID := c.Param("uuid")

		// Get the OCI registry handler
		handler, err := registryService.GetRegistry("oci")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get registry handler"})
			return
		}

		ociRegistry, ok := handler.(*oci.Registry)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid registry handler type"})
			return
		}

		// Get upload session status
		session, err := ociRegistry.GetBlobUploadStatus(sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload session not found"})
			return
		}

		c.Header("Range", fmt.Sprintf("0-%d", session.Size-1))
		c.Header("Docker-Upload-UUID", sessionID)
		c.Status(http.StatusNoContent)

		log.Debug().
			Str("session_id", sessionID).
			Int64("current_size", session.Size).
			Msg("Retrieved blob upload status")
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

		artifacts, _, err := registryService.List(ctx, filter)
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

		artifacts, _, err := registryService.List(ctx, filter)
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

// handleOCIRequest is a catch-all handler that routes OCI requests based on path patterns
func handleOCIRequest(registryService *registry.Service, authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		method := c.Request.Method

		// Remove leading slash from path
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Handle base endpoint for Docker CLI compatibility
		if path == "" || path == "/" {
			if method == "GET" {
				c.Header("Docker-Distribution-API-Version", "registry/2.0")
				c.JSON(http.StatusOK, gin.H{
					"name": "Lodestone OCI Registry",
				})
				return
			}
		}

		// Handle Docker authentication endpoints
		if path == "auth" {
			if method == "GET" || method == "POST" {
				handleDockerAuth(authService)(c)
				return
			}
		}

		if path == "token" {
			if method == "GET" || method == "POST" {
				handleDockerToken(authService)(c)
				return
			}
		}

		// Handle catalog endpoint specifically
		if path == "_catalog" {
			if method == "GET" {
				middleware.OptionalAuthMiddleware(authService)(c)
				if c.IsAborted() {
					return
				}
				handleOCICatalog(registryService)(c)
				return
			}
		}

		// Parse the path to determine the operation
		if strings.HasSuffix(path, "/tags/list") {
			// Repository tags list
			if method == "GET" {
				middleware.OptionalAuthMiddleware(authService)(c)
				if c.IsAborted() {
					return
				}
				handleOCITagsListCatchAll(registryService)(c)
				return
			}
		} else if strings.Contains(path, "/manifests/") {
			// Manifest operations
			middleware.OptionalAuthMiddleware(authService)(c)
			if c.IsAborted() && (method == "PUT" || method == "DELETE") {
				return
			}
			if method == "PUT" || method == "DELETE" {
				middleware.AuthMiddleware(authService)(c)
				if c.IsAborted() {
					return
				}
			}
			handleOCIManifestCatchAll(registryService)(c)
			return
		} else if strings.Contains(path, "/blobs/uploads/") {
			// Blob upload operations
			middleware.AuthMiddleware(authService)(c)
			if c.IsAborted() {
				return
			}
			handleOCIBlobUploadCatchAll(registryService)(c)
			return
		} else if strings.Contains(path, "/blobs/") {
			// Blob operations
			middleware.OptionalAuthMiddleware(authService)(c)
			if c.IsAborted() && method == "DELETE" {
				return
			}
			if method == "DELETE" {
				middleware.AuthMiddleware(authService)(c)
				if c.IsAborted() {
					return
				}
			}
			handleOCIBlobCatchAll(registryService)(c)
			return
		}

		// If no pattern matches, return 404
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	}
}

// Helper function to extract repository name from catch-all path
func extractRepositoryNameFromPath(path, pattern string) (string, string, bool) {
	// For patterns like "repo/name/manifests/tag" -> ("repo/name", "tag")
	if strings.Contains(path, pattern) {
		parts := strings.Split(path, pattern)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

func handleOCITagsListCatchAll(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Extract repository name (remove "/tags/list" suffix)
		name := strings.TrimSuffix(path, "/tags/list")

		// Set the name parameter for compatibility with existing handler
		c.Params = append(c.Params, gin.Param{Key: "name", Value: name})

		handleOCITagsList(registryService)(c)
	}
}

func handleOCIManifestCatchAll(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Extract repository name and reference from path like "repo/name/manifests/tag"
		name, reference, ok := extractRepositoryNameFromPath(path, "/manifests/")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid manifest path"})
			return
		}

		// Set parameters for compatibility with existing handlers
		c.Params = append(c.Params, gin.Param{Key: "name", Value: name})
		c.Params = append(c.Params, gin.Param{Key: "reference", Value: reference})

		method := c.Request.Method
		switch method {
		case "GET":
			handleOCIManifestGet(registryService)(c)
		case "PUT":
			handleOCIManifestPut(registryService)(c)
		case "DELETE":
			handleOCIManifestDelete(registryService)(c)
		case "HEAD":
			handleOCIManifestHead(registryService)(c)
		default:
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		}
	}
}

func handleOCIBlobCatchAll(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Extract repository name and digest from path like "repo/name/blobs/sha256:..."
		name, digest, ok := extractRepositoryNameFromPath(path, "/blobs/")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid blob path"})
			return
		}

		// Set parameters for compatibility with existing handlers
		c.Params = append(c.Params, gin.Param{Key: "name", Value: name})
		c.Params = append(c.Params, gin.Param{Key: "digest", Value: digest})

		method := c.Request.Method
		switch method {
		case "GET":
			handleOCIBlobGet(registryService)(c)
		case "HEAD":
			handleOCIBlobHead(registryService)(c)
		case "DELETE":
			handleOCIBlobDelete(registryService)(c)
		default:
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		}
	}
}

func handleOCIBlobUploadCatchAll(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		method := c.Request.Method

		if strings.HasSuffix(path, "/blobs/uploads/") {
			// Start upload: POST /repo/name/blobs/uploads/
			if method == "POST" {
				name := strings.TrimSuffix(path, "/blobs/uploads/")
				c.Params = append(c.Params, gin.Param{Key: "name", Value: name})
				handleOCIBlobUploadStart(registryService)(c)
				return
			}
		} else if strings.Contains(path, "/blobs/uploads/") {
			// Upload operations with UUID: /repo/name/blobs/uploads/uuid
			name, uuid, ok := extractRepositoryNameFromPath(path, "/blobs/uploads/")
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload path"})
				return
			}

			c.Params = append(c.Params, gin.Param{Key: "name", Value: name})
			c.Params = append(c.Params, gin.Param{Key: "uuid", Value: uuid})

			switch method {
			case "PATCH":
				handleOCIBlobUploadChunk(registryService)(c)
			case "PUT":
				handleOCIBlobUploadComplete(registryService)(c)
			case "DELETE":
				handleOCIBlobUploadCancel(registryService)(c)
			case "GET":
				handleOCIBlobUploadStatus(registryService)(c)
			default:
				c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
			}
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	}
}

// Docker authentication handlers
func handleDockerAuth(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Handle Docker login challenge
		// Docker CLI sends credentials via Basic Auth
		username, password, hasAuth := c.Request.BasicAuth()

		if !hasAuth {
			// Return authentication challenge
			c.Header("WWW-Authenticate", `Basic realm="Lodestone Docker Registry"`)
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusUnauthorized, gin.H{
				"errors": []gin.H{
					{
						"code":    "UNAUTHORIZED",
						"message": "authentication required",
					},
				},
			})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "docker_auth", true)

		// Try to authenticate with username/password as API key
		// Docker login typically uses username as anything and password as API key
		var user *types.User
		var err error

		if password != "" {
			// Try password as API key first
			user, _, err = authService.ValidateAPIKey(ctx, password)
			if err != nil {
				// If API key validation fails, try traditional login
				loginReq := &types.LoginRequest{
					Username: username,
					Password: password,
				}
				token, loginErr := authService.Login(ctx, loginReq)
				if loginErr == nil {
					// Login successful, validate the token to get user
					user, err = authService.ValidateToken(ctx, token.Token)
				} else {
					err = loginErr
				}
			}
		}

		if err != nil {
			c.Header("WWW-Authenticate", `Basic realm="Lodestone Docker Registry"`)
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusUnauthorized, gin.H{
				"errors": []gin.H{
					{
						"code":    "UNAUTHORIZED",
						"message": "invalid credentials",
					},
				},
			})
			return
		}

		// Authentication successful
		c.Header("Docker-Distribution-API-Version", "registry/2.0")

		// Log successful authentication
		log.Info().
			Str("username", username).
			Str("user_id", user.ID.String()).
			Msg("Docker authentication successful")

		c.JSON(http.StatusOK, gin.H{
			"access_token": password, // Return the API key as access token
			"scope":        "repository:*:*",
			"issued_at":    time.Now().Format(time.RFC3339),
			"expires_in":   3600,
		})
	}
}

func handleDockerToken(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Handle Docker token requests (OAuth2-like flow)
		service := c.Query("service")
		scope := c.Query("scope")

		// Check for Basic Auth
		username, password, hasAuth := c.Request.BasicAuth()

		if !hasAuth {
			c.Header("WWW-Authenticate", `Basic realm="Lodestone Docker Registry"`)
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusUnauthorized, gin.H{
				"errors": []gin.H{
					{
						"code":    "UNAUTHORIZED",
						"message": "authentication required",
					},
				},
			})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "docker_token", true)

		// Authenticate using API key
		var user *types.User
		var err error

		if password != "" {
			// Try password as API key first
			user, _, err = authService.ValidateAPIKey(ctx, password)
			if err != nil {
				// If API key validation fails, try traditional login
				loginReq := &types.LoginRequest{
					Username: username,
					Password: password,
				}
				token, loginErr := authService.Login(ctx, loginReq)
				if loginErr == nil {
					user, err = authService.ValidateToken(ctx, token.Token)
				} else {
					err = loginErr
				}
			}
		}

		if err != nil {
			c.Header("WWW-Authenticate", `Basic realm="Lodestone Docker Registry"`)
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusUnauthorized, gin.H{
				"errors": []gin.H{
					{
						"code":    "UNAUTHORIZED",
						"message": "invalid credentials",
					},
				},
			})
			return
		}

		// Generate a simple token response
		// In a full implementation, this would be a proper JWT with the requested scope
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.JSON(http.StatusOK, gin.H{
			"token":        password, // Use API key as token
			"access_token": password,
			"expires_in":   3600,
			"issued_at":    time.Now().Format(time.RFC3339),
			"scope":        scope,
		})

		// Log successful authentication
		log.Info().
			Str("username", username).
			Str("service", service).
			Str("scope", scope).
			Str("user_id", user.ID.String()).
			Msg("Docker token issued successfully")
	}
}
