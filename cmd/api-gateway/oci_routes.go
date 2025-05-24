package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// OCI/Docker registry route setup
func setupOCIRoutes(api *gin.RouterGroup, registryService *registry.Service) {
	oci := api.Group("/v2")
	
	// Docker Registry API v2
	oci.GET("/", handleOCIAPIVersion())
	oci.GET("/:name/manifests/:reference", handleOCIManifestGet(registryService))
	oci.PUT("/:name/manifests/:reference", authenticateAPIKey(), handleOCIManifestPut(registryService))
	oci.DELETE("/:name/manifests/:reference", authenticateAPIKey(), handleOCIManifestDelete(registryService))
	oci.HEAD("/:name/manifests/:reference", handleOCIManifestHead(registryService))
	
	// Blob operations
	oci.GET("/:name/blobs/:digest", handleOCIBlobGet(registryService))
	oci.HEAD("/:name/blobs/:digest", handleOCIBlobHead(registryService))
	oci.DELETE("/:name/blobs/:digest", authenticateAPIKey(), handleOCIBlobDelete(registryService))
	
	// Blob upload operations
	oci.POST("/:name/blobs/uploads/", authenticateAPIKey(), handleOCIBlobUploadStart(registryService))
	oci.PATCH("/:name/blobs/uploads/:uuid", authenticateAPIKey(), handleOCIBlobUploadChunk(registryService))
	oci.PUT("/:name/blobs/uploads/:uuid", authenticateAPIKey(), handleOCIBlobUploadComplete(registryService))
	oci.DELETE("/:name/blobs/uploads/:uuid", authenticateAPIKey(), handleOCIBlobUploadCancel(registryService))
	
	// Catalog and tags
	oci.GET("/_catalog", handleOCICatalog(registryService))
	oci.GET("/:name/tags/list", handleOCITagsList(registryService))
}

func handleOCIAPIVersion() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.JSON(http.StatusOK, map[string]interface{}{})
	}
}

func handleOCIManifestGet(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")
		
		// Handle nested repository names (e.g., library/nginx)
		if wildcard := c.Param("*"); wildcard != "" {
			name = name + "/" + wildcard
		}
		
		artifact, content, err := registryService.Download(c.Request.Context(), "oci", name, reference)
		if err != nil {
			logrus.WithError(err).Error("Failed to get OCI manifest")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "MANIFEST_UNKNOWN",
						"message": "manifest unknown",
						"detail": map[string]interface{}{
							"name": name,
							"tag":  reference,
						},
					},
				},
			})
			return
		}
		
		// Set appropriate headers
		c.Header("Content-Type", artifact.ContentType)
		c.Header("Docker-Content-Digest", artifact.SHA256)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		
		// Stream the content to the response
		defer content.Close()
		c.Stream(func(w io.Writer) bool {
			_, err := io.Copy(w, content)
			return err == nil
		})
	}
}

func handleOCIManifestPut(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")
		
		_, exists := c.Get("user")
		if !exists {
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "UNAUTHORIZED",
						"message": "authentication required",
					},
				},
			})
			return
		}
		
		// Read manifest content
		content, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "MANIFEST_INVALID",
						"message": "manifest invalid",
					},
				},
			})
			return
		}
		
		contentType := c.GetHeader("Content-Type")
		if contentType == "" {
			contentType = "application/vnd.oci.image.manifest.v1+json"
		}
		
		// Get user from context
		userObj := c.MustGet("user").(*types.User)
		
		// Create a reader from the content
		contentReader := bytes.NewReader(content)
		
		// Upload using the registry service
		_, err = registryService.Upload(c.Request.Context(), "oci", name, reference, contentReader, userObj.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to upload OCI manifest")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "UNKNOWN",
						"message": "unknown error occurred",
					},
				},
			})
			return
		}
		
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Header("Location", fmt.Sprintf("/v2/%s/manifests/%s", name, reference))
		c.Status(http.StatusCreated)
	}
}

func handleOCIManifestDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")
		
		user := c.MustGet("user").(*types.User)
		err := registryService.Delete(c.Request.Context(), "oci", name, reference, user.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to delete OCI manifest")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "MANIFEST_UNKNOWN",
						"message": "manifest unknown",
					},
				},
			})
			return
		}
		
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Status(http.StatusAccepted)
	}
}

func handleOCIManifestHead(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		reference := c.Param("reference")
		
		artifact, _, err := registryService.Download(c.Request.Context(), "oci", name, reference)
		if err != nil {
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.Status(http.StatusNotFound)
			return
		}
		
		c.Header("Content-Type", artifact.ContentType)
		c.Header("Docker-Content-Digest", artifact.SHA256)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
		c.Status(http.StatusOK)
	}
}

func handleOCIBlobGet(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		digest := c.Param("digest")
		
		artifact, content, err := registryService.Download(c.Request.Context(), "oci", name, digest)
		if err != nil {
			logrus.WithError(err).Error("Failed to get OCI blob")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "BLOB_UNKNOWN",
						"message": "blob unknown to registry",
					},
				},
			})
			return
		}
		
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Docker-Content-Digest", digest)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
		
		// Stream the content to the response
		defer content.Close()
		c.Stream(func(w io.Writer) bool {
			_, err := io.Copy(w, content)
			return err == nil
		})
	}
}

func handleOCIBlobHead(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		digest := c.Param("digest")
		
		artifact, _, err := registryService.Download(c.Request.Context(), "oci", name, digest)
		if err != nil {
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.Status(http.StatusNotFound)
			return
		}
		
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Docker-Content-Digest", digest)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
		c.Status(http.StatusOK)
	}
}

func handleOCIBlobDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		digest := c.Param("digest")
		
		user := c.MustGet("user").(*types.User)
		err := registryService.Delete(c.Request.Context(), "oci", name, digest, user.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to delete OCI blob")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "BLOB_UNKNOWN",
						"message": "blob unknown to registry",
					},
				},
			})
			return
		}
		
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Status(http.StatusAccepted)
	}
}

func handleOCIBlobUploadStart(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		
		// Generate upload UUID
		uploadUUID := generateUploadUUID()
		
		// Store upload session (in practice, would use Redis or similar)
		// For now, just return the upload URL
		
		uploadURL := fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, uploadUUID)
		
		c.Header("Location", uploadURL)
		c.Header("Range", "0-0")
		c.Header("Docker-Upload-UUID", uploadUUID)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Status(http.StatusAccepted)
	}
}

func handleOCIBlobUploadChunk(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		uuid := c.Param("uuid")
		
		// Handle chunked upload
		// In practice, this would append to a temporary file
		
		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, uuid))
		c.Header("Range", "0-1023")
		c.Header("Docker-Upload-UUID", uuid)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Status(http.StatusAccepted)
	}
}

func handleOCIBlobUploadComplete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		_ = c.Param("uuid") // UUID for upload session
		digest := c.Query("digest")
		
		if digest == "" {
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "DIGEST_INVALID",
						"message": "provided digest did not match uploaded content",
					},
				},
			})
			return
		}
		
		// Read final chunk content
		content, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "UNKNOWN",
						"message": "failed to read upload content",
					},
				},
			})
			return
		}
		
		// Get user from context
		userObj := c.MustGet("user").(*types.User)
		
		// Create a reader from the content
		contentReader := bytes.NewReader(content)
		
		// Upload using the registry service  
		_, err = registryService.Upload(c.Request.Context(), "oci", name, digest, contentReader, userObj.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to complete OCI blob upload")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "UNKNOWN",
						"message": "failed to complete upload",
					},
				},
			})
			return
		}
		
		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
		c.Header("Docker-Content-Digest", digest)
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Status(http.StatusCreated)
	}
}

func handleOCIBlobUploadCancel(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Cancel upload session
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Status(http.StatusNoContent)
	}
}

func handleOCICatalog(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		n := c.DefaultQuery("n", "100")
		_ = c.Query("last") // For pagination (not implemented yet)
		
		nInt, _ := strconv.Atoi(n)
		
		filter := &types.ArtifactFilter{
			Registry: "oci",
			Limit:    nInt,
		}
		
		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			logrus.WithError(err).Error("Failed to get OCI catalog")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "UNKNOWN",
						"message": "failed to get catalog",
					},
				},
			})
			return
		}
		
		// Extract unique repository names
		repos := make(map[string]bool)
		for _, artifact := range artifacts {
			repos[artifact.Name] = true
		}
		
		repositories := make([]string, 0, len(repos))
		for repo := range repos {
			repositories = append(repositories, repo)
		}
		
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.JSON(http.StatusOK, map[string]interface{}{
			"repositories": repositories,
		})
	}
}

func handleOCITagsList(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		
		filter := &types.ArtifactFilter{
			Name:     name,
			Registry: "oci",
		}
		
		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			logrus.WithError(err).Error("Failed to get OCI tags")
			c.Header("Docker-Distribution-API-Version", "registry/2.0")
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"code":    "NAME_UNKNOWN",
						"message": "repository name not known to registry",
					},
				},
			})
			return
		}
		
		tags := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			if !strings.HasPrefix(artifact.Version, "sha256:") {
				tags = append(tags, artifact.Version)
			}
		}
		
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.JSON(http.StatusOK, map[string]interface{}{
			"name": name,
			"tags": tags,
		})
	}
}

// generateUploadUUID generates a UUID for OCI upload sessions
func generateUploadUUID() string {
	return uuid.New().String()
}
