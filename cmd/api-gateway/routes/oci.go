package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
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

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")

		// For OCI, the reference could be a tag or digest
		var version string
		if strings.HasPrefix(reference, "sha256:") {
			// It's a digest, need to look up by SHA256
			filter := &types.ArtifactFilter{
				Name:     name,
				Registry: "oci",
			}

			artifacts, _, err := registryService.List(ctx, filter)
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

		artifact, content, err := registryService.Download(ctx, "oci", name, version)
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

		name := extractRepositoryName(c)
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

		_, err := registryService.Upload(ctx, "oci", name, reference, c.Request.Body, user.ID)
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

		err := registryService.Delete(ctx, "oci", name, reference, user.ID)
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

		artifact, _, err := registryService.Download(ctx, "oci", name, reference)
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

		artifacts, _, err := registryService.List(ctx, filter)
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

		_, content, err := registryService.Download(ctx, "oci", name, targetArtifact.Version)
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

		artifacts, _, err := registryService.List(ctx, filter)
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

// Blob upload handlers - simplified implementations
func handleOCIBlobUploadStart(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		name := extractRepositoryName(c)

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
		_ = c.Param("uuid") // Upload session UUID - not needed for our implementation
		digest := c.Query("digest")

		if digest == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "digest parameter required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "oci")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		_, err := registryService.Upload(ctx, "oci", name, digest, c.Request.Body, user.ID)
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
