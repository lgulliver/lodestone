package routes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
	"github.com/rs/zerolog/log"
)

// NPMRoutes sets up npm registry routes
func NPMRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	npm := api.Group("/npm")

	// Package metadata and download
	npm.GET("/:name", handleNPMPackageInfo(registryService))
	npm.GET("/:name/:version", handleNPMPackageVersion(registryService))
	npm.GET("/@:scope/:name", handleNPMScopedPackageInfo(registryService))
	npm.GET("/@:scope/:name/:version", handleNPMScopedPackageVersion(registryService))

	// Package tarball download
	npm.GET("/:name/-/:filename", handleNPMDownload(registryService))
	npm.GET("/@:scope/:name/-/:filename", handleNPMScopedDownload(registryService))

	// Package publish (requires authentication)
	npm.PUT("/:name", middleware.AuthMiddleware(authService), handleNPMPublish(registryService))
	npm.PUT("/@:scope/:name", middleware.AuthMiddleware(authService), handleNPMScopedPublish(registryService))

	// Package delete (requires authentication)
	npm.DELETE("/:name/-rev/:rev", middleware.AuthMiddleware(authService), handleNPMDelete(registryService))
	npm.DELETE("/@:scope/:name/-rev/:rev", middleware.AuthMiddleware(authService), handleNPMScopedDelete(registryService))

	// Search
	npm.GET("/-/v1/search", handleNPMSearch(registryService))
}

// computeArtifactSHA1 computes the SHA1 hash of an artifact's content
func computeArtifactSHA1(ctx context.Context, registryService *registry.Service, artifact *types.Artifact) (string, error) {
	// Download the artifact content
	_, content, err := registryService.Download(ctx, artifact.Registry, artifact.Name, artifact.Version)
	if err != nil {
		return "", fmt.Errorf("failed to download artifact for SHA1 computation: %w", err)
	}
	defer content.Close()

	// Read all content and compute SHA1
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return "", fmt.Errorf("failed to read artifact content: %w", err)
	}

	return utils.ComputeSHA1(contentBytes), nil
}

func handleNPMPackageInfo(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("name")
		if packageName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package name required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		filter := &types.ArtifactFilter{
			Name:     packageName,
			Registry: "npm",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get package info"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}

		// Build npm package info response
		versions := make(map[string]interface{})
		distTags := map[string]string{
			"latest": "",
		}

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

			// Compute SHA1 hash for npm compatibility
			shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
			if err != nil {
				log.Error().Err(err).
					Str("package", artifact.Name).
					Str("version", artifact.Version).
					Msg("failed to compute SHA1 hash, falling back to SHA256")
				shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
			}

			versions[artifact.Version] = gin.H{
				"name":        artifact.Name,
				"version":     artifact.Version,
				"description": description,
				"author":      author,
				"dist": gin.H{
					"shasum":  shasum,
					"tarball": fmt.Sprintf("http://%s/api/v1/npm/%s/-/%s-%s.tgz", c.Request.Host, artifact.Name, artifact.Name, artifact.Version),
				},
			}

			// Set latest version (simple logic - last in list)
			distTags["latest"] = artifact.Version
		}

		c.JSON(http.StatusOK, gin.H{
			"name":      packageName,
			"versions":  versions,
			"dist-tags": distTags,
			"time":      gin.H{}, // TODO: implement time tracking
		})
	}
}

func handleNPMPackageVersion(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("name")
		version := c.Param("version")

		if packageName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package name and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		artifact, _, err := registryService.Download(ctx, "npm", packageName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "package version not found"})
			return
		}

		var description, author string
		if artifact.Metadata != nil {
			if desc, ok := artifact.Metadata["description"].(string); ok {
				description = desc
			}
			if auth, ok := artifact.Metadata["author"].(string); ok {
				author = auth
			}
		}

		// Compute SHA1 hash for npm compatibility
		shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
		if err != nil {
			log.Error().Err(err).
				Str("package", artifact.Name).
				Str("version", artifact.Version).
				Msg("failed to compute SHA1 hash, falling back to SHA256")
			shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
		}

		c.JSON(http.StatusOK, gin.H{
			"name":        artifact.Name,
			"version":     artifact.Version,
			"description": description,
			"author":      author,
			"dist": gin.H{
				"shasum":  shasum,
				"tarball": fmt.Sprintf("http://%s/api/v1/npm/%s/-/%s-%s.tgz", c.Request.Host, artifact.Name, artifact.Name, artifact.Version),
			},
		})
	}
}

func handleNPMScopedPackageInfo(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope := c.Param("scope")
		name := c.Param("name")
		packageName := fmt.Sprintf("@%s/%s", scope, name)

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		filter := &types.ArtifactFilter{
			Name:     packageName,
			Registry: "npm",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get package info"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}

		// Similar to handleNPMPackageInfo but with scoped package name
		versions := make(map[string]interface{})
		distTags := map[string]string{
			"latest": "",
		}

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

			// Compute SHA1 hash for npm compatibility
			shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
			if err != nil {
				log.Error().Err(err).
					Str("package", artifact.Name).
					Str("version", artifact.Version).
					Msg("failed to compute SHA1 hash, falling back to SHA256")
				shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
			}

			versions[artifact.Version] = gin.H{
				"name":        artifact.Name,
				"version":     artifact.Version,
				"description": description,
				"author":      author,
				"dist": gin.H{
					"shasum":  shasum,
					"tarball": fmt.Sprintf("http://%s/api/v1/npm/@%s/%s/-/%s-%s.tgz", c.Request.Host, scope, name, name, artifact.Version),
				},
			}
			distTags["latest"] = artifact.Version
		}

		c.JSON(http.StatusOK, gin.H{
			"name":      packageName,
			"versions":  versions,
			"dist-tags": distTags,
			"time":      gin.H{},
		})
	}
}

func handleNPMScopedPackageVersion(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope := c.Param("scope")
		name := c.Param("name")
		version := c.Param("version")
		packageName := fmt.Sprintf("@%s/%s", scope, name)

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		artifact, _, err := registryService.Download(ctx, "npm", packageName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "package version not found"})
			return
		}

		var description, author string
		if artifact.Metadata != nil {
			if desc, ok := artifact.Metadata["description"].(string); ok {
				description = desc
			}
			if auth, ok := artifact.Metadata["author"].(string); ok {
				author = auth
			}
		}

		// Compute SHA1 hash for npm compatibility
		shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
		if err != nil {
			log.Error().Err(err).
				Str("package", artifact.Name).
				Str("version", artifact.Version).
				Msg("failed to compute SHA1 hash, falling back to SHA256")
			shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
		}

		c.JSON(http.StatusOK, gin.H{
			"name":        artifact.Name,
			"version":     artifact.Version,
			"description": description,
			"author":      author,
			"dist": gin.H{
				"shasum":  shasum,
				"tarball": fmt.Sprintf("http://%s/api/v1/npm/@%s/%s/-/%s-%s.tgz", c.Request.Host, scope, name, name, artifact.Version),
			},
		})
	}
}

func handleNPMDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("name")
		filename := c.Param("filename")

		// Extract version from filename (format: packagename-version.tgz)
		version := strings.TrimPrefix(filename, packageName+"-")
		version = strings.TrimSuffix(version, ".tgz")

		log.Info().
			Str("package", packageName).
			Str("version", version).
			Str("filename", filename).
			Msg("downloading npm package")

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		artifact, content, err := registryService.Download(ctx, "npm", packageName, version)
		if err != nil {
			log.Error().Err(err).
				Str("package", packageName).
				Str("version", version).
				Msg("failed to download npm package")
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream package content"})
			return
		}
	}
}

func handleNPMScopedDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope := c.Param("scope")
		name := c.Param("name")
		filename := c.Param("filename")
		packageName := fmt.Sprintf("@%s/%s", scope, name)

		// Extract version from filename
		version := strings.TrimPrefix(filename, name+"-")
		version = strings.TrimSuffix(version, ".tgz")

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		artifact, content, err := registryService.Download(ctx, "npm", packageName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream package content"})
			return
		}
	}
}

func handleNPMPublish(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var publishData map[string]interface{}
		if err := c.ShouldBindJSON(&publishData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		packageName := c.Param("name")
		ctx := context.WithValue(c.Request.Context(), "registry", "npm")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		log.Info().
			Str("package_name", packageName).
			Str("user_id", user.ID.String()).
			Msg("Processing NPM publish request")

		// Extract package.json and tarball from publish data
		attachments, ok := publishData["_attachments"].(map[string]interface{})
		if !ok {
			log.Error().Msg("Missing _attachments in publish data")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing package attachments"})
			return
		}

		log.Info().
			Int("attachment_count", len(attachments)).
			Msg("Found attachments in publish data")

		// Process each attachment (tarball)
		for filename, attachment := range attachments {
			log.Info().
				Str("filename", filename).
				Msg("Processing attachment")

			attachmentData, ok := attachment.(map[string]interface{})
			if !ok {
				log.Warn().
					Str("filename", filename).
					Msg("Attachment data is not a map, skipping")
				continue
			}

			data, ok := attachmentData["data"].(string)
			if !ok {
				log.Warn().
					Str("filename", filename).
					Msg("Attachment data field is not a string, skipping")
				continue
			}

			log.Info().
				Str("filename", filename).
				Int("base64_length", len(data)).
				Msg("Found base64 data in attachment")

			// Decode base64 tarball data
			tarballData, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				log.Error().
					Err(err).
					Str("filename", filename).
					Msg("Failed to decode base64 data")
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 data"})
				return
			}

			log.Info().
				Str("filename", filename).
				Int("tarball_size", len(tarballData)).
				Msg("Successfully decoded tarball data")

			// Extract package.json from tarball
			packageJSON, version, err := extractPackageJSONFromTarball(tarballData)
			if err != nil {
				log.Error().
					Err(err).
					Str("filename", filename).
					Msg("Failed to extract package.json from tarball")
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to extract package.json: %v", err)})
				return
			}

			log.Info().
				Str("filename", filename).
				Str("extracted_name", packageJSON["name"].(string)).
				Str("extracted_version", version).
				Msg("Successfully extracted package.json")

			// Validate package name matches
			if packageJSON["name"] != packageName {
				log.Error().
					Str("expected_name", packageName).
					Str("actual_name", packageJSON["name"].(string)).
					Msg("Package name mismatch")
				c.JSON(http.StatusBadRequest, gin.H{"error": "package name mismatch"})
				return
			}

			log.Info().
				Str("package_name", packageName).
				Str("version", version).
				Msg("Starting artifact upload to registry service")

			// Upload the package
			artifact, err := registryService.Upload(ctx, "npm", packageName, version, bytes.NewReader(tarballData), user.ID)
			if err != nil {
				log.Error().
					Err(err).
					Str("package_name", packageName).
					Str("version", version).
					Msg("Failed to upload package to registry service")
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to upload package: %v", err)})
				return
			}

			log.Info().
				Str("package_name", packageName).
				Str("version", version).
				Str("artifact_id", artifact.ID.String()).
				Msg("Successfully uploaded package to registry service")

			c.JSON(http.StatusCreated, gin.H{
				"ok":  true,
				"id":  packageName,
				"rev": fmt.Sprintf("1-%s", version), // Simplified revision
			})

			_ = filename // Mark as used
			_ = artifact // Mark as used
			return
		}

		log.Error().Msg("No valid package data found in attachments")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid package data found"})
	}
}

func handleNPMScopedPublish(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		scope := c.Param("scope")
		name := c.Param("name")
		packageName := fmt.Sprintf("@%s/%s", scope, name)

		var publishData map[string]interface{}
		if err := c.ShouldBindJSON(&publishData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// Extract package.json and tarball from publish data
		attachments, ok := publishData["_attachments"].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing package attachments"})
			return
		}

		// Process each attachment (tarball)
		for filename, attachment := range attachments {
			attachmentData, ok := attachment.(map[string]interface{})
			if !ok {
				continue
			}

			data, ok := attachmentData["data"].(string)
			if !ok {
				continue
			}

			// Decode base64 tarball data
			tarballData, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 data"})
				return
			}

			// Extract package.json from tarball
			packageJSON, version, err := extractPackageJSONFromTarball(tarballData)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to extract package.json: %v", err)})
				return
			}

			// Validate package name matches
			if packageJSON["name"] != packageName {
				c.JSON(http.StatusBadRequest, gin.H{"error": "package name mismatch"})
				return
			}

			// Upload the package
			artifact, err := registryService.Upload(ctx, "npm", packageName, version, bytes.NewReader(tarballData), user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to upload package: %v", err)})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"ok":  true,
				"id":  packageName,
				"rev": fmt.Sprintf("1-%s", version), // Simplified revision
			})

			_ = filename // Mark as used
			_ = artifact // Mark as used
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid package data found"})
	}
}

func handleNPMDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		packageName := c.Param("name")
		rev := c.Param("rev") // npm revision parameter

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// For now, delete all versions of the package
		// In a real implementation, you'd parse the rev to determine what to delete
		_ = rev

		err := registryService.Delete(ctx, "npm", packageName, "", user.ID) // Empty version = delete all
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete package"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
		})
	}
}

func handleNPMScopedDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		scope := c.Param("scope")
		name := c.Param("name")
		packageName := fmt.Sprintf("@%s/%s", scope, name)
		rev := c.Param("rev")

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		_ = rev

		err := registryService.Delete(ctx, "npm", packageName, "", user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete package"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
		})
	}
}

func handleNPMSearch(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		text := c.Query("text")
		_ = c.DefaultQuery("size", "20") // TODO: implement pagination
		_ = c.DefaultQuery("from", "0")  // TODO: implement pagination

		ctx := context.WithValue(c.Request.Context(), "registry", "npm")

		filter := &types.ArtifactFilter{
			Registry: "npm",
		}

		if text != "" {
			filter.Name = text // Simple name-based search
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		// Convert to npm search response format
		objects := make([]gin.H, 0, len(artifacts))
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

			objects = append(objects, gin.H{
				"package": gin.H{
					"name":        artifact.Name,
					"version":     artifact.Version,
					"description": description,
					"author":      gin.H{"name": author},
				},
				"score": gin.H{
					"final":  1.0,
					"detail": gin.H{},
				},
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"objects": objects,
			"total":   len(objects),
			"time":    "0ms",
		})
	}
}

// extractPackageJSONFromTarball extracts and parses package.json from an npm tarball
func extractPackageJSONFromTarball(tarballData []byte) (map[string]interface{}, string, error) {
	// Create a gzip reader
	gzipReader, err := gzip.NewReader(bytes.NewReader(tarballData))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Look for package.json in the tarball
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to read tar header: %w", err)
		}

		// Look for package.json file (could be in package/ directory)
		if strings.HasSuffix(header.Name, "package.json") {
			// Read the package.json content
			packageJSONBytes, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read package.json: %w", err)
			}

			// Parse package.json
			var packageJSON map[string]interface{}
			if err := json.Unmarshal(packageJSONBytes, &packageJSON); err != nil {
				return nil, "", fmt.Errorf("failed to parse package.json: %w", err)
			}

			// Extract version
			version, ok := packageJSON["version"].(string)
			if !ok {
				return nil, "", fmt.Errorf("package.json missing version field")
			}

			return packageJSON, version, nil
		}
	}

	return nil, "", fmt.Errorf("package.json not found in tarball")
}
