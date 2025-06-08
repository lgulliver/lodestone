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
	"time"

	"github.com/Masterminds/semver/v3"
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

	// Package metadata and download - requires authentication
	npm.GET("/:name", middleware.AuthMiddleware(authService), handleNPMPackageInfo(registryService))
	npm.GET("/:name/:version", middleware.AuthMiddleware(authService), handleNPMPackageVersion(registryService))
	npm.GET("/@:scope/:name", middleware.AuthMiddleware(authService), handleNPMScopedPackageInfo(registryService))
	npm.GET("/@:scope/:name/:version", middleware.AuthMiddleware(authService), handleNPMScopedPackageVersion(registryService))

	// Package tarball download - requires authentication
	npm.GET("/:name/-/:filename", middleware.AuthMiddleware(authService), handleNPMDownload(registryService))
	npm.GET("/@:scope/:name/-/:filename", middleware.AuthMiddleware(authService), handleNPMScopedDownload(registryService))

	// Package publish (requires authentication)
	npm.PUT("/:name", middleware.AuthMiddleware(authService), handleNPMPublish(registryService))
	npm.PUT("/@:scope/:name", middleware.AuthMiddleware(authService), handleNPMScopedPublish(registryService))

	// Package delete (requires authentication)
	npm.DELETE("/:name/-rev/:rev", middleware.AuthMiddleware(authService), handleNPMDelete(registryService))
	npm.DELETE("/@:scope/:name/-rev/:rev", middleware.AuthMiddleware(authService), handleNPMScopedDelete(registryService))

	// Search - requires authentication
	npm.GET("/-/v1/search", middleware.AuthMiddleware(authService), handleNPMSearch(registryService))
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

// generateTarballURL creates the correct URL for package tarballs
// This centralizes the URL generation logic to avoid inconsistencies
func generateTarballURL(c *gin.Context, packageName, version string) string {
	// Handle scoped packages (@scope/name)
	if strings.HasPrefix(packageName, "@") {
		// Split @scope/name into components
		parts := strings.SplitN(packageName, "/", 2)
		if len(parts) == 2 {
			scope := strings.TrimPrefix(parts[0], "@")
			name := parts[1]
			return fmt.Sprintf("http://%s/api/v1/npm/@%s/%s/-/%s-%s.tgz",
				c.Request.Host, scope, name, name, version)
		}
	}

	// Handle regular packages
	return fmt.Sprintf("http://%s/api/v1/npm/%s/-/%s-%s.tgz",
		c.Request.Host, packageName, packageName, version)
}

// processTimes creates a standardized time map for NPM packages
func processTimes(artifacts []*types.Artifact) map[string]string {
	times := make(map[string]string)
	var firstCreated, lastModified time.Time

	// Initialize with empty time
	firstCreated = time.Now()

	for _, artifact := range artifacts {
		// Track version-specific times
		versionTime := artifact.CreatedAt.Format(time.RFC3339)
		times[artifact.Version] = versionTime

		// Track earliest creation time
		if artifact.CreatedAt.Before(firstCreated) {
			firstCreated = artifact.CreatedAt
		}

		// Track latest modification time
		if artifact.UpdatedAt.After(lastModified) {
			lastModified = artifact.UpdatedAt
		}

		// Check for additional time info in metadata
		if artifact.Metadata != nil {
			if timeInfo, ok := artifact.Metadata["time"].(map[string]string); ok {
				for ver, ts := range timeInfo {
					// Don't override version times with metadata if already set from artifact
					if _, exists := times[ver]; !exists {
						times[ver] = ts
					}
				}
			}
		}
	}

	// Set created/modified times
	times["created"] = firstCreated.Format(time.RFC3339)
	times["modified"] = lastModified.Format(time.RFC3339)

	return times
}

// processDistTags builds the dist-tags object based on artifact metadata and semver rules
func processDistTags(artifacts []*types.Artifact, versionList []string) map[string]string {
	distTags := make(map[string]string)
	latestVersionIsTagged := false

	// First pass: extract explicitly defined tags from artifact metadata
	for _, artifact := range artifacts {
		if artifact.Metadata != nil {
			if dtags, ok := artifact.Metadata["dist-tags"].(map[string]interface{}); ok {
				for tag, ver := range dtags {
					if verStr, ok := ver.(string); ok {
						distTags[tag] = verStr
						if tag == "latest" {
							latestVersionIsTagged = true
						}
					}
				}
			}
		}
	}

	// Second pass: if no latest tag was set, determine it based on semver rules
	if !latestVersionIsTagged && len(versionList) > 0 {
		// Filter out prerelease versions for latest tag
		stableVersions := make([]string, 0, len(versionList))
		for _, v := range versionList {
			if !utils.IsPrerelease(v) {
				stableVersions = append(stableVersions, v)
			}
		}

		latestVersion := ""
		if len(stableVersions) > 0 {
			// Use the latest stable version
			latestVersion = utils.GetLatestVersion(stableVersions)
		} else if len(versionList) > 0 {
			// If no stable versions, use the latest prerelease
			latestVersion = utils.GetLatestVersion(versionList)
		}

		if latestVersion != "" {
			distTags["latest"] = latestVersion
		}
	} else if len(distTags) == 0 && len(versionList) > 0 {
		// Fallback if no tags at all
		distTags["latest"] = utils.GetLatestVersion(versionList)
	}

	return distTags
}

// buildVersionObject creates a standardized version object for NPM package responses
func buildVersionObject(c *gin.Context, artifact *types.Artifact, shasum string) gin.H {
	versionObj := gin.H{
		"name":    artifact.Name,
		"version": artifact.Version,
		"_id":     artifact.Name + "@" + artifact.Version,
		"dist": gin.H{
			"shasum":  shasum,
			"tarball": generateTarballURL(c, artifact.Name, artifact.Version),
		},
	}

	// Add fields from metadata if available
	if artifact.Metadata != nil {
		addFieldIfExists(versionObj, artifact.Metadata, "description", "description")
		addFieldIfExists(versionObj, artifact.Metadata, "author", "author")
		addFieldIfExists(versionObj, artifact.Metadata, "homepage", "homepage")
		addFieldIfExists(versionObj, artifact.Metadata, "keywords", "keywords")
		addFieldIfExists(versionObj, artifact.Metadata, "bugs", "bugs")
		addFieldIfExists(versionObj, artifact.Metadata, "license", "license")
		addFieldIfExists(versionObj, artifact.Metadata, "repository", "repository")
		addFieldIfExists(versionObj, artifact.Metadata, "engines", "engines")
		addFieldIfExists(versionObj, artifact.Metadata, "dependencies", "dependencies")
		addFieldIfExists(versionObj, artifact.Metadata, "devDependencies", "devDependencies")
		addFieldIfExists(versionObj, artifact.Metadata, "peerDependencies", "peerDependencies")
		addFieldIfExists(versionObj, artifact.Metadata, "deprecated", "deprecated")
	}

	return versionObj
}

// addFieldIfExists adds a field to a map if it exists in the source map
func addFieldIfExists(target gin.H, source map[string]interface{}, sourceKey, targetKey string) {
	if value, ok := source[sourceKey]; ok && value != nil {
		target[targetKey] = value
	}
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

		// Process artifacts
		versions := make(map[string]interface{})
		versionList := make([]string, 0, len(artifacts))

		// Get list of all versions
		for _, artifact := range artifacts {
			versionList = append(versionList, artifact.Version)
		}

		// Sort versions for consistent ordering
		utils.SortSemver(versionList)

		// Process time information for all artifacts
		times := processTimes(artifacts)

		// Process artifacts and build version objects
		for _, artifact := range artifacts {
			// Compute SHA1 hash for npm compatibility
			shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
			if err != nil {
				log.Error().Err(err).
					Str("package", artifact.Name).
					Str("version", artifact.Version).
					Msg("failed to compute SHA1 hash, falling back to SHA256")
				shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
			}

			// Build standardized version object
			versionObj := buildVersionObject(c, artifact, shasum)
			versions[artifact.Version] = versionObj
		}

		// Process distribution tags
		distTags := processDistTags(artifacts, versionList)

		c.JSON(http.StatusOK, gin.H{
			"name":      packageName,
			"versions":  versions,
			"dist-tags": distTags,
			"time":      times,
			"modified":  time.Now().Format(time.RFC3339),
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

		// Compute SHA1 hash for npm compatibility
		shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
		if err != nil {
			log.Error().Err(err).
				Str("package", artifact.Name).
				Str("version", artifact.Version).
				Msg("failed to compute SHA1 hash, falling back to SHA256")
			shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
		}

		// Build version response using our helper function
		versionObj := buildVersionObject(c, artifact, shasum)

		// Add time information
		versionObj["time"] = artifact.CreatedAt.Format(time.RFC3339)

		c.JSON(http.StatusOK, versionObj)
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

		// Process artifacts
		versions := make(map[string]interface{})
		versionList := make([]string, 0, len(artifacts))

		// Get list of all versions
		for _, artifact := range artifacts {
			versionList = append(versionList, artifact.Version)
		}

		// Sort versions for consistent ordering
		utils.SortSemver(versionList)

		// Process time information for all artifacts
		times := processTimes(artifacts)

		// Process artifacts and build version objects
		for _, artifact := range artifacts {
			// Compute SHA1 hash for npm compatibility
			shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
			if err != nil {
				log.Error().Err(err).
					Str("package", artifact.Name).
					Str("version", artifact.Version).
					Msg("failed to compute SHA1 hash, falling back to SHA256")
				shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
			}

			// Build standardized version object
			versionObj := buildVersionObject(c, artifact, shasum)
			versions[artifact.Version] = versionObj
		}

		// Process distribution tags
		distTags := processDistTags(artifacts, versionList)

		c.JSON(http.StatusOK, gin.H{
			"name":      packageName,
			"versions":  versions,
			"dist-tags": distTags,
			"time":      times,
			"modified":  time.Now().Format(time.RFC3339),
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

		// Compute SHA1 hash for npm compatibility
		shasum, err := computeArtifactSHA1(ctx, registryService, artifact)
		if err != nil {
			log.Error().Err(err).
				Str("package", artifact.Name).
				Str("version", artifact.Version).
				Msg("failed to compute SHA1 hash, falling back to SHA256")
			shasum = artifact.SHA256 // fallback to SHA256 if SHA1 computation fails
		}

		// Build version response using our helper function
		versionObj := buildVersionObject(c, artifact, shasum)

		// Add time information
		versionObj["time"] = artifact.CreatedAt.Format(time.RFC3339)

		c.JSON(http.StatusOK, versionObj)
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

			// Extract dist-tags from publish data if available
			distTags := make(map[string]string)
			if publishDataDistTags, ok := publishData["dist-tags"].(map[string]interface{}); ok {
				for tag, ver := range publishDataDistTags {
					if verStr, ok := ver.(string); ok {
						distTags[tag] = verStr
						log.Info().
							Str("package", packageName).
							Str("tag", tag).
							Str("version", verStr).
							Msg("Found dist-tag in publish data")
					}
				}
			}

			// If no dist-tags provided, we need to make an intelligent decision
			// about whether to set this version as "latest"
			if len(distTags) == 0 {
				// Check if this is a stable version (not a prerelease)
				if !utils.IsPrerelease(version) {
					// Check if there's a current latest version in the registry
					existingVersions := make([]string, 0)
					existingArtifacts, _, _ := registryService.List(ctx, &types.ArtifactFilter{
						Registry: "npm",
						Name:     packageName,
					})

					latestTagExists := false
					latestVersion := ""

					for _, art := range existingArtifacts {
						existingVersions = append(existingVersions, art.Version)

						// Look for existing latest tag in metadata
						if art.Metadata != nil {
							if dtags, ok := art.Metadata["dist-tags"].(map[string]interface{}); ok {
								for tag, ver := range dtags {
									if tag == "latest" {
										latestTagExists = true
										if verStr, ok := ver.(string); ok {
											latestVersion = verStr
										}
									}
								}
							}
						}
					}

					// If there's an existing latest tag, compare the versions
					if latestTagExists && latestVersion != "" {
						// Only take over "latest" tag if this version is greater
						// than the current latest
						compareResult := utils.CompareVersions(version, latestVersion)
						if compareResult > 0 { // New version is greater
							distTags["latest"] = version
							log.Info().
								Str("package", packageName).
								Str("version", version).
								Str("previous_latest", latestVersion).
								Msg("Setting new version as latest based on semver comparison")
						}
					} else {
						// No existing latest tag, set this as latest
						distTags["latest"] = version
						log.Info().
							Str("package", packageName).
							Str("version", version).
							Msg("Setting as latest (first stable version)")
					}
				} else {
					// This is a prerelease version, don't set as latest
					// but consider setting as tag based on prerelease identifier
					prereleaseId := getPrereleaseIdentifier(version)
					if prereleaseId != "" {
						distTags[prereleaseId] = version
						log.Info().
							Str("package", packageName).
							Str("version", version).
							Str("tag", prereleaseId).
							Msg("Setting prerelease tag based on identifier")
					}
				}
			}

			// Store dist-tags in metadata
			if packageJSON["metadata"] == nil {
				packageJSON["metadata"] = make(map[string]interface{})
			}

			// Add distTags to metadata
			if len(distTags) > 0 {
				packageJSON["dist-tags"] = distTags
			}

			// Store time information in metadata
			now := time.Now().Format(time.RFC3339)
			timeInfo := map[string]string{
				"created":  now,
				"modified": now,
				version:    now,
			}
			packageJSON["time"] = timeInfo

			log.Info().
				Str("package_name", packageName).
				Str("version", version).
				Msg("Starting artifact upload to registry service")

			// Upload the package with enhanced metadata
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

		log.Info().
			Str("package_name", packageName).
			Str("user_id", user.ID.String()).
			Msg("Processing NPM scoped package publish request")

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

			// Extract dist-tags from publish data if available
			distTags := make(map[string]string)
			if publishDataDistTags, ok := publishData["dist-tags"].(map[string]interface{}); ok {
				for tag, ver := range publishDataDistTags {
					if verStr, ok := ver.(string); ok {
						distTags[tag] = verStr
						log.Info().
							Str("package", packageName).
							Str("tag", tag).
							Str("version", verStr).
							Msg("Found dist-tag in publish data")
					}
				}
			}

			// If no dist-tags provided, we need to make an intelligent decision
			// about whether to set this version as "latest"
			if len(distTags) == 0 {
				// Check if this is a stable version (not a prerelease)
				if !utils.IsPrerelease(version) {
					// Check if there's a current latest version in the registry
					existingVersions := make([]string, 0)
					existingArtifacts, _, _ := registryService.List(ctx, &types.ArtifactFilter{
						Registry: "npm",
						Name:     packageName,
					})

					latestTagExists := false
					latestVersion := ""

					for _, art := range existingArtifacts {
						existingVersions = append(existingVersions, art.Version)

						// Look for existing latest tag in metadata
						if art.Metadata != nil {
							if dtags, ok := art.Metadata["dist-tags"].(map[string]interface{}); ok {
								for tag, ver := range dtags {
									if tag == "latest" {
										latestTagExists = true
										if verStr, ok := ver.(string); ok {
											latestVersion = verStr
										}
									}
								}
							}
						}
					}

					// If there's an existing latest tag, compare the versions
					if latestTagExists && latestVersion != "" {
						// Only take over "latest" tag if this version is greater
						// than the current latest
						compareResult := utils.CompareVersions(version, latestVersion)
						if compareResult > 0 { // New version is greater
							distTags["latest"] = version
							log.Info().
								Str("package", packageName).
								Str("version", version).
								Str("previous_latest", latestVersion).
								Msg("Setting new version as latest based on semver comparison")
						}
					} else {
						// No existing latest tag, set this as latest
						distTags["latest"] = version
						log.Info().
							Str("package", packageName).
							Str("version", version).
							Msg("Setting as latest (first stable version)")
					}
				} else {
					// This is a prerelease version, don't set as latest
					// but consider setting as tag based on prerelease identifier
					prereleaseId := getPrereleaseIdentifier(version)
					if prereleaseId != "" {
						distTags[prereleaseId] = version
						log.Info().
							Str("package", packageName).
							Str("version", version).
							Str("tag", prereleaseId).
							Msg("Setting prerelease tag based on identifier")
					}
				}
			}

			// Store dist-tags in metadata
			if packageJSON["metadata"] == nil {
				packageJSON["metadata"] = make(map[string]interface{})
			}

			// Add distTags to metadata
			if len(distTags) > 0 {
				packageJSON["dist-tags"] = distTags
			}

			// Store time information in metadata
			now := time.Now().Format(time.RFC3339)
			timeInfo := map[string]string{
				"created":  now,
				"modified": now,
				version:    now,
			}
			packageJSON["time"] = timeInfo

			log.Info().
				Str("package_name", packageName).
				Str("version", version).
				Msg("Starting artifact upload to registry service")

			// Upload the package with enhanced metadata
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

// getPrereleaseIdentifier extracts the prerelease identifier from a semver version
// For example:
// - "1.0.0-beta.1" returns "beta"
// - "2.0.0-alpha.3" returns "alpha"
// - "3.0.0-rc.5" returns "rc"
// - "4.0.0" returns ""
func getPrereleaseIdentifier(version string) string {
	// Parse the version to ensure it's valid semver
	sv, err := semver.NewVersion(version)
	if err != nil {
		return ""
	}

	// Get the prerelease string
	prerelease := sv.Prerelease()
	if prerelease == "" {
		return ""
	}

	// Extract the identifier part (before the first dot or digit)
	identifierEnd := 0
	for i, c := range prerelease {
		if c == '.' || (c >= '0' && c <= '9') {
			identifierEnd = i
			break
		}
	}

	if identifierEnd > 0 {
		return prerelease[:identifierEnd]
	}

	return prerelease
}
