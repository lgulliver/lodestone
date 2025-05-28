package routes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// NuGetRoutes sets up NuGet package manager routes
func NuGetRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	nuget := api.Group("/nuget")

	// NuGet v3 Service Index (root metadata endpoint)
	nuget.GET("/v3/index.json", handleNuGetServiceIndex())

	// NuGet v3 API routes
	// Package content (flat container)
	nuget.GET("/v3-flatcontainer/:id/index.json", handleNuGetPackageVersions(registryService))
	nuget.GET("/v3-flatcontainer/:id/:version/:filename", middleware.OptionalAuthMiddleware(authService), handleNuGetDownload(registryService))

	// Package publish (requires authentication)
	nuget.PUT("/api/v2/package", middleware.AuthMiddleware(authService), handleNuGetUpload(registryService))
	nuget.DELETE("/api/v2/package/:id/:version", middleware.AuthMiddleware(authService), handleNuGetDelete(registryService))

	// Symbol package endpoints (requires authentication)
	nuget.PUT("/api/v2/symbolpackage", middleware.AuthMiddleware(authService), handleNuGetSymbolUpload(registryService))
	nuget.GET("/symbols/:id/:version/:filename", middleware.OptionalAuthMiddleware(authService), handleNuGetSymbolDownload(registryService))

	// Search API
	nuget.GET("/v3/search", handleNuGetSearch(registryService))

	// Package registration (metadata)
	nuget.GET("/v3/registration/:id/index.json", handleNuGetPackageMetadata(registryService))
}

func handleNuGetServiceIndex() gin.HandlerFunc {
	return func(c *gin.Context) {
		// NuGet v3 Service Index response
		// This tells NuGet clients where to find various services
		baseURL := fmt.Sprintf("%s://%s/api/v1/nuget",
			c.Request.Header.Get("X-Forwarded-Proto"), c.Request.Host)
		if baseURL[:4] != "http" {
			if c.Request.TLS != nil {
				baseURL = "https://" + c.Request.Host + "/api/v1/nuget"
			} else {
				baseURL = "http://" + c.Request.Host + "/api/v1/nuget"
			}
		}

		serviceIndex := gin.H{
			"version": "3.0.0",
			"resources": []gin.H{
				{
					"@id":     baseURL + "/v3-flatcontainer/",
					"@type":   "PackageBaseAddress/3.0.0",
					"comment": "Base URL of where NuGet packages are stored, in the format https://api.nuget.org/v3-flatcontainer/{id-lower}/{version-lower}/{id-lower}.{version-lower}.nupkg",
				},
				{
					"@id":     baseURL + "/v3/search",
					"@type":   "SearchQueryService",
					"comment": "Query endpoint of NuGet Search service (primary)",
				},
				{
					"@id":     baseURL + "/v3/registration/",
					"@type":   "RegistrationsBaseUrl",
					"comment": "Base URL of NuGet package registration service (primary)",
				},
				{
					"@id":     baseURL + "/api/v2/package",
					"@type":   "PackagePublish/2.0.0",
					"comment": "NuGet package publish endpoint",
				},
				{
					"@id":     baseURL + "/api/v2/symbolpackage",
					"@type":   "SymbolPackagePublish/4.9.0",
					"comment": "NuGet symbol package publish endpoint",
				},
				{
					"@id":     baseURL + "/symbols/",
					"@type":   "SymbolPackageBaseAddress/4.9.0",
					"comment": "Base URL of NuGet symbol server",
				},
			},
		}

		c.JSON(http.StatusOK, serviceIndex)
	}
}

func handleNuGetPackageVersions(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		if packageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")

		filter := &types.ArtifactFilter{
			Name:     packageID,
			Registry: "nuget",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list packages"})
			return
		}

		versions := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			versions = append(versions, artifact.Version)
		}

		c.JSON(http.StatusOK, gin.H{
			"versions": versions,
		})
	}
}

func handleNuGetDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		version := c.Param("version")
		filename := c.Param("filename")

		if packageID == "" || version == "" || filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID, version, and filename required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")

		artifact, content, err := registryService.Download(ctx, "nuget", packageID, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}

		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		if artifact.Size > 0 {
			c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
		}

		defer content.Close()
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream package content"})
			return
		}
	}
}

func handleNuGetUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		file, header, err := c.Request.FormFile("package")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package file required"})
			return
		}
		defer file.Close()

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// Extract package name and version from filename
		// NuGet packages follow the pattern: PackageName.Version.nupkg
		filename := header.Filename
		if !strings.HasSuffix(filename, ".nupkg") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package file extension"})
			return
		}

		nameVersion := strings.TrimSuffix(filename, ".nupkg")
		parts := strings.Split(nameVersion, ".")
		if len(parts) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package filename format"})
			return
		}

		// Find where version starts (versions are typically numeric)
		versionStartIndex := len(parts) - 1
		for i := len(parts) - 1; i >= 1; i-- {
			if _, err := strconv.Atoi(string(parts[i][0])); err == nil {
				versionStartIndex = i
			} else {
				break
			}
		}

		packageName := strings.Join(parts[:versionStartIndex], ".")
		version := strings.Join(parts[versionStartIndex:], ".")

		_, err = registryService.Upload(ctx, "nuget", packageName, version, file, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "package uploaded successfully",
		})
	}
}

func handleNuGetDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		packageID := c.Param("id")
		version := c.Param("version")

		if packageID == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID and version required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		err := registryService.Delete(ctx, "nuget", packageID, version, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete package"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "package deleted successfully",
		})
	}
}

func handleNuGetSearch(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		skip := c.DefaultQuery("skip", "0")
		take := c.DefaultQuery("take", "20")

		skipInt, _ := strconv.Atoi(skip)
		takeInt, _ := strconv.Atoi(take)

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")

		filter := &types.ArtifactFilter{
			Registry: "nuget",
			Limit:    takeInt,
			Offset:   skipInt,
		}

		if query != "" {
			filter.Name = query // Simple name-based search for now
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		// Convert to NuGet search response format
		results := make([]gin.H, 0, len(artifacts))
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

			results = append(results, gin.H{
				"id":             artifact.Name,
				"version":        artifact.Version,
				"description":    description,
				"authors":        []string{author},
				"totalDownloads": 0, // TODO: implement download counting
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"totalHits": len(results),
			"data":      results,
		})
	}
}

func handleNuGetPackageMetadata(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		if packageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package ID required"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")

		filter := &types.ArtifactFilter{
			Name:     packageID,
			Registry: "nuget",
		}

		artifacts, _, err := registryService.List(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get package metadata"})
			return
		}

		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}

		// Build NuGet registration response
		items := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			var description string
			if artifact.Metadata != nil {
				if desc, ok := artifact.Metadata["description"].(string); ok {
					description = desc
				}
			}

			items = append(items, gin.H{
				"catalogEntry": gin.H{
					"id":          artifact.Name,
					"version":     artifact.Version,
					"description": description,
					"published":   artifact.CreatedAt,
				},
				"packageContent": fmt.Sprintf("/nuget/v3-flatcontainer/%s/%s/%s.%s.nupkg",
					strings.ToLower(artifact.Name), artifact.Version,
					strings.ToLower(artifact.Name), artifact.Version),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"count": len(items),
			"items": []gin.H{
				{
					"items": items,
				},
			},
		})
	}
}

// handleNuGetSymbolUpload handles symbol package uploads (.snupkg files)
func handleNuGetSymbolUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := middleware.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Get the uploaded file
		file, err := c.FormFile("package")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no package file provided"})
			return
		}

		// Open the file
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
			return
		}
		defer src.Close()

		// Read the content
		content, err := io.ReadAll(src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
			return
		}

		// Parse the filename to extract package name and version
		// Expected format: packagename.version.snupkg
		filename := file.Filename
		if !strings.HasSuffix(strings.ToLower(filename), ".snupkg") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol package must have .snupkg extension"})
			return
		}

		// Remove .snupkg extension and parse name.version
		nameVersion := strings.TrimSuffix(filename, ".snupkg")
		parts := strings.Split(nameVersion, ".")
		if len(parts) < 4 { // name.major.minor.patch at minimum
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid symbol package filename format"})
			return
		}

		// Find the last occurrence that looks like a version (contains digits)
		var packageName, version string
		for i := len(parts) - 3; i >= 0; i-- {
			if len(parts) > i+2 {
				candidateVersion := strings.Join(parts[i:], ".")
				// Check if this looks like a version
				if isValidVersion(candidateVersion) {
					packageName = strings.Join(parts[:i], ".")
					version = candidateVersion
					break
				}
			}
		}

		if packageName == "" || version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse package name and version from filename"})
			return
		}

		// Upload the symbol package directly
		result, err := registryService.Upload(c.Request.Context(), "nuget", packageName, version, bytes.NewReader(content), user.ID)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				c.JSON(http.StatusConflict, gin.H{"error": "symbol package already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":     "Symbol package uploaded successfully",
			"packageName": result.Name,
			"version":     result.Version,
			"id":          result.ID,
		})
	}
}

// handleNuGetSymbolDownload handles symbol package downloads
func handleNuGetSymbolDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("id")
		version := c.Param("version")
		filename := c.Param("filename")

		// Validate that this is a symbol package request
		if !strings.HasSuffix(strings.ToLower(filename), ".snupkg") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "not a symbol package file"})
			return
		}

		// Find the symbol package artifact
		filter := &types.ArtifactFilter{
			Name:     packageName,
			Registry: "nuget",
		}

		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search for symbol package"})
			return
		}

		// Look for the symbol package specifically
		var symbolArtifact *types.Artifact
		for _, artifact := range artifacts {
			if artifact.Metadata != nil {
				if packageType, exists := artifact.Metadata["packageType"]; exists && packageType == "SymbolsPackage" {
					symbolArtifact = artifact
					break
				}
			}
		}

		if symbolArtifact == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "symbol package not found"})
			return
		}

		// Download the symbol package
		_, content, err := registryService.Download(c.Request.Context(), "nuget", packageName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "symbol package not found"})
			return
		}
		defer content.Close()

		// Set appropriate headers for symbol package download
		c.Header("Content-Type", "application/vnd.nuget.symbolpackage")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		// Stream the content
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			// Log error but don't send JSON response as headers are already sent
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
}

// isValidVersion checks if a string looks like a semantic version
func isValidVersion(version string) bool {
	// Simple check for version pattern: digits, dots, and optional pre-release/build metadata
	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	return versionRegex.MatchString(version)
}
