package routes

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/registry/registries/nuget"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
)

// NuSpecMetadata represents the metadata in a .nuspec file
type NuSpecMetadata struct {
	ID      string `xml:"id"`
	Version string `xml:"version"`
	Title   string `xml:"title"`
	Authors string `xml:"authors"`
	Owners  string `xml:"owners"`
}

// NuSpec represents the structure of a .nuspec file
type NuSpec struct {
	XMLName  xml.Name       `xml:"package"`
	Metadata NuSpecMetadata `xml:"metadata"`
}

// extractNuGetPackageInfo extracts package name and version from .nupkg file contents
func extractNuGetPackageInfo(fileContent []byte) (string, string, error) {
	// Create a byte reader for the zip content
	reader := bytes.NewReader(fileContent)
	zipReader, err := zip.NewReader(reader, int64(len(fileContent)))
	if err != nil {
		return "", "", fmt.Errorf("failed to read nupkg as zip: %w", err)
	}

	// Look for .nuspec file in the zip
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, ".nuspec") {
			log.Info().Str("nuspec_file", file.Name).Msg("Found .nuspec file in package")

			rc, err := file.Open()
			if err != nil {
				return "", "", fmt.Errorf("failed to open nuspec file: %w", err)
			}
			defer rc.Close()

			// Read and parse the .nuspec XML
			nuspecContent, err := io.ReadAll(rc)
			if err != nil {
				return "", "", fmt.Errorf("failed to read nuspec content: %w", err)
			}

			var nuspec NuSpec
			err = xml.Unmarshal(nuspecContent, &nuspec)
			if err != nil {
				return "", "", fmt.Errorf("failed to parse nuspec XML: %w", err)
			}

			if nuspec.Metadata.ID == "" || nuspec.Metadata.Version == "" {
				return "", "", fmt.Errorf("missing required metadata in nuspec file")
			}

			log.Info().
				Str("package_id", nuspec.Metadata.ID).
				Str("version", nuspec.Metadata.Version).
				Msg("Extracted package metadata from .nuspec file")

			return nuspec.Metadata.ID, nuspec.Metadata.Version, nil
		}
	}

	return "", "", fmt.Errorf("no .nuspec file found in package")
}

// NuGetRoutes sets up NuGet package manager routes
func NuGetRoutes(api *gin.RouterGroup, registryService *registry.Service, authService *auth.Service) {
	nuget := api.Group("/nuget")

	// NuGet v3 Service Index (root metadata endpoint)
	nuget.GET("/v3/index.json", handleNuGetServiceIndex())

	// NuGet v3 API routes
	// Package content (flat container) - discovery endpoints allow optional auth, downloads require auth
	nuget.GET("/v3-flatcontainer/:id/index.json", middleware.OptionalAuthMiddleware(authService), handleNuGetPackageVersions(registryService))
	nuget.GET("/v3-flatcontainer/:id/:version/:filename", middleware.AuthMiddleware(authService), handleNuGetDownload(registryService))

	// Package publish (requires authentication) - NuGet v2 API
	nuget.PUT("/v2/package", middleware.AuthMiddleware(authService), handleNuGetUpload(registryService))
	nuget.PUT("/v2/package/", middleware.AuthMiddleware(authService), handleNuGetUpload(registryService))
	nuget.DELETE("/v2/package/:id/:version", middleware.AuthMiddleware(authService), handleNuGetDelete(registryService))

	// Symbol package endpoints (requires authentication) - NuGet v2 API
	nuget.PUT("/v2/symbolpackage", middleware.AuthMiddleware(authService), handleNuGetSymbolUpload(registryService))
	nuget.GET("/symbols/:id/:version/:filename", middleware.AuthMiddleware(authService), handleNuGetSymbolDownload(registryService))

	// Search API - allows optional auth for discovery but requires auth for private packages
	nuget.GET("/v3/search", middleware.OptionalAuthMiddleware(authService), handleNuGetSearch(registryService))

	// Package registration (metadata) - allow optional auth for discovery
	nuget.GET("/v3/registration/:id/index.json", middleware.OptionalAuthMiddleware(authService), handleNuGetPackageMetadata(registryService))
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
					"@id":     baseURL + "/v3/search",
					"@type":   "SearchQueryService/3.0.0-rc",
					"comment": "Query endpoint of NuGet Search service (versioned)",
				},
				{
					"@id":     baseURL + "/v3/registration/",
					"@type":   "RegistrationsBaseUrl",
					"comment": "Base URL of NuGet package registration service (primary)",
				},
				{
					"@id":     baseURL + "/v3/registration/",
					"@type":   "RegistrationsBaseUrl/3.0.0-rc",
					"comment": "Base URL of NuGet package registration service (versioned)",
				},
				{
					"@id":     baseURL + "/v2/package",
					"@type":   "PackagePublish/2.0.0",
					"comment": "NuGet package publish endpoint",
				},
				{
					"@id":     baseURL + "/v2/symbolpackage",
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
			// Log error but don't send JSON response as headers are already sent
			log.Error().Err(err).Msg("Failed to stream package content")
			c.AbortWithStatus(http.StatusInternalServerError)
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

		// Log request details for debugging
		contentType := c.GetHeader("Content-Type")
		log.Info().
			Str("method", c.Request.Method).
			Str("content_type", contentType).
			Int64("content_length", c.Request.ContentLength).
			Msg("Processing NuGet upload request")

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		var fileContent []byte
		var filename string
		var err error

		// Handle different upload methods: multipart form data or raw binary
		if strings.HasPrefix(contentType, "multipart/form-data") {
			// Handle multipart form data (web uploads)
			err := c.Request.ParseMultipartForm(32 << 20) // 32MB max
			if err != nil {
				log.Error().Err(err).Msg("Failed to parse multipart form")
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form", "details": err.Error()})
				return
			}

			// Try different possible field names for the package file
			var file multipart.File
			var header *multipart.FileHeader
			for _, fieldName := range []string{"package", "file"} {
				file, header, err = c.Request.FormFile(fieldName)
				if err == nil {
					break
				}
			}

			// If no file found with known field names, try to get the first file
			if err != nil && c.Request.MultipartForm != nil && len(c.Request.MultipartForm.File) > 0 {
				for _, files := range c.Request.MultipartForm.File {
					if len(files) > 0 {
						header = files[0]
						file, err = header.Open()
						break
					}
				}
			}

			if err != nil {
				log.Error().Err(err).Msg("No package file found in upload")
				c.JSON(http.StatusBadRequest, gin.H{"error": "no package file found in upload"})
				return
			}
			defer file.Close()

			filename = header.Filename
			fileContent, err = io.ReadAll(file)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read package file content")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read package file"})
				return
			}
		} else {
			// Handle raw binary upload (NuGet CLI)
			fileContent, err = io.ReadAll(c.Request.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read request body")
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
				return
			}

			// For raw uploads, we'll extract the filename from package metadata
			// For now, use a temporary filename that will be corrected later
			filename = "package.nupkg"
		}

		log.Info().
			Str("filename", filename).
			Int("content_size", len(fileContent)).
			Msg("Processing NuGet package")

		// Validate it's a .nupkg file (for multipart uploads)
		if strings.Contains(filename, ".") && !strings.HasSuffix(strings.ToLower(filename), ".nupkg") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package file extension"})
			return
		}

		// Extract package name and version from .nupkg file contents
		packageName, version, err := extractNuGetPackageInfo(fileContent)
		if err != nil {
			log.Error().Err(err).Str("filename", filename).Msg("Failed to extract package metadata from .nupkg file")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid package format: %v", err)})
			return
		}

		log.Info().
			Str("filename", filename).
			Str("packageName", packageName).
			Str("version", version).
			Msg("Successfully extracted package information from .nupkg file")

		// Create a reader from the file content for upload
		contentReader := bytes.NewReader(fileContent)

		_, err = registryService.Upload(ctx, "nuget", packageName, version, contentReader, user.ID)
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

		// Use ILIKE in service layer for case-insensitive search
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

		// Build NuGet registration response according to the official spec
		// https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource
		baseURL := fmt.Sprintf("%s://%s/api/v1/nuget",
			c.Request.Header.Get("X-Forwarded-Proto"), c.Request.Host)
		if baseURL[:4] != "http" {
			if c.Request.TLS != nil {
				baseURL = "https://" + c.Request.Host + "/api/v1/nuget"
			} else {
				baseURL = "http://" + c.Request.Host + "/api/v1/nuget"
			}
		}

		catalogEntries := make([]gin.H, 0, len(artifacts))
		for _, artifact := range artifacts {
			var description, authors string
			if artifact.Metadata != nil {
				if desc, ok := artifact.Metadata["description"].(string); ok {
					description = desc
				}
				if auth, ok := artifact.Metadata["authors"].(string); ok {
					authors = auth
				}
			}
			if authors == "" {
				authors = "Unknown"
			}

			catalogEntries = append(catalogEntries, gin.H{
				"@id": fmt.Sprintf("%s/v3/registration/%s/%s.json",
					baseURL, strings.ToLower(packageID), artifact.Version),
				"@type":           "Package",
				"commitId":        "00000000-0000-0000-0000-000000000000",
				"commitTimeStamp": artifact.CreatedAt,
				"catalogEntry": gin.H{
					"@id": fmt.Sprintf("%s/v3/registration/%s/%s.json",
						baseURL, strings.ToLower(packageID), artifact.Version),
					"@type":       "PackageDetails",
					"authors":     authors,
					"description": description,
					"id":          artifact.Name, // Use the original case-preserved name from the artifact
					"version":     artifact.Version,
					"published":   artifact.CreatedAt,
					"packageContent": fmt.Sprintf("%s/v3-flatcontainer/%s/%s/%s.%s.nupkg",
						baseURL, strings.ToLower(packageID), artifact.Version,
						strings.ToLower(artifact.Name), artifact.Version),
				},
				"packageContent": fmt.Sprintf("%s/v3-flatcontainer/%s/%s/%s.%s.nupkg",
					baseURL, strings.ToLower(packageID), artifact.Version,
					strings.ToLower(artifact.Name), artifact.Version),
				"registration": fmt.Sprintf("%s/v3/registration/%s/index.json",
					baseURL, strings.ToLower(packageID)),
			})
		}

		response := gin.H{
			"@id": fmt.Sprintf("%s/v3/registration/%s/index.json",
				baseURL, strings.ToLower(packageID)),
			"@type": []string{"catalog:CatalogRoot", "PackageRegistration", "catalog:Permalink"},
			"count": 1,
			"items": []gin.H{
				{
					"@id": fmt.Sprintf("%s/v3/registration/%s/index.json#page/1.0.0/%s",
						baseURL, strings.ToLower(packageID), artifacts[len(artifacts)-1].Version),
					"@type": "catalog:CatalogPage",
					"count": len(catalogEntries),
					"items": catalogEntries,
					"lower": artifacts[0].Version,
					"upper": artifacts[len(artifacts)-1].Version,
					"parent": fmt.Sprintf("%s/v3/registration/%s/index.json",
						baseURL, strings.ToLower(packageID)),
				},
			},
		}

		c.JSON(http.StatusOK, response)
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

		log.Info().
			Str("method", c.Request.Method).
			Str("content_type", c.GetHeader("Content-Type")).
			Int64("content_length", c.Request.ContentLength).
			Msg("Processing NuGet symbol upload request")

		var content []byte
		var filename string
		var err error

		contentType := c.GetHeader("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			// Handle multipart form upload (web interface)
			log.Info().Msg("Processing multipart form symbol upload")
			file, err := c.FormFile("package")
			if err != nil {
				log.Error().Err(err).Msg("Failed to get form file")
				c.JSON(http.StatusBadRequest, gin.H{"error": "no package file provided"})
				return
			}
			filename = file.Filename
			src, err := file.Open()
			if err != nil {
				log.Error().Err(err).Msg("Failed to open uploaded file")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
				return
			}
			defer src.Close()
			content, err = io.ReadAll(src)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read uploaded file")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
				return
			}
		} else {
			// Accept both application/octet-stream and other raw uploads (dotnet CLI)
			log.Info().Msg("Processing raw binary symbol upload")
			content, err = io.ReadAll(c.Request.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read request body")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
				return
			}
			filename = ""
		}

		log.Info().
			Str("filename", filename).
			Int("content_size", len(content)).
			Msg("Symbol package content processed")

		// Always extract package name and version from content
		packageName, version, err := extractSymbolPackageInfo(content)
		if err != nil {
			log.Error().Err(err).Msg("Failed to extract package info from symbol package")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to extract package information: %v", err)})
			return
		}

		if filename == "" {
			filename = fmt.Sprintf("%s.%s.snupkg", packageName, version)
		}

		if !strings.HasSuffix(strings.ToLower(filename), ".snupkg") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol package must have .snupkg extension"})
			return
		}

		// Create artifact with symbol package metadata
		symbolArtifactName := packageName + ".symbols"
		artifact := &types.Artifact{
			Name:        symbolArtifactName,
			Version:     version,
			Registry:    "nuget",
			PublishedBy: user.ID,
			Size:        int64(len(content)),
			ContentType: "application/vnd.nuget.symbolpackage",
			Metadata: map[string]interface{}{
				"packageType":   "symbols",
				"parentPackage": packageName,
				"contentType":   "application/vnd.nuget.symbolpackage",
				"filename":      filename,
			},
		}

		nugetRegistry := &nuget.Registry{}
		artifact.StoragePath = nugetRegistry.GenerateSymbolStoragePath(packageName, version)

		if err := nugetRegistry.Validate(artifact, content); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("validation failed: %v", err)})
			return
		}

		// Check if symbol package already exists
		var existingSymbol types.Artifact
		symbolStoragePath := nugetRegistry.GenerateSymbolStoragePath(packageName, version)
		if err := registryService.DB.Where("storage_path = ? AND registry = ?", symbolStoragePath, "nuget").First(&existingSymbol).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "symbol package already exists"})
			return
		}

		if err := registryService.Storage.Store(c.Request.Context(), artifact.StoragePath, bytes.NewReader(content), "application/vnd.nuget.symbolpackage"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to store symbol package: %v", err)})
			return
		}

		if err := registryService.DB.Create(artifact).Error; err != nil {
			registryService.Storage.Delete(c.Request.Context(), artifact.StoragePath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save symbol package metadata: %v", err)})
			return
		}

		// NuGet expects an empty body and 201 Created for successful push
		c.Status(http.StatusCreated)
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

		// Find the symbol package artifact using the symbol-specific name
		symbolArtifactName := packageName + ".symbols"
		filter := &types.ArtifactFilter{
			Name:     symbolArtifactName,
			Registry: "nuget",
		}

		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search for symbol package"})
			return
		}

		// Look for the symbol package with the correct version
		var symbolArtifact *types.Artifact
		for _, artifact := range artifacts {
			if artifact.Version == version && artifact.Metadata != nil {
				if packageType, exists := artifact.Metadata["packageType"]; exists && packageType == "symbols" {
					symbolArtifact = artifact
					break
				}
			}
		}

		if symbolArtifact == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "symbol package not found"})
			return
		}

		// Download the symbol package using the registry service to get proper metadata
		_, content, err := registryService.Download(c.Request.Context(), "nuget", symbolArtifactName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "symbol package not found"})
			return
		}
		defer content.Close()

		// Set appropriate headers for symbol package download
		c.Header("Content-Type", "application/vnd.nuget.symbolpackage")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		// Set Content-Length from the artifact size
		if symbolArtifact.Size > 0 {
			c.Header("Content-Length", strconv.FormatInt(symbolArtifact.Size, 10))
		}

		// Stream the content
		_, err = io.Copy(c.Writer, content)
		if err != nil {
			// Log error but don't send JSON response as headers are already sent
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
}

// extractSymbolPackageFilename extracts the filename from symbol package content
// by reading the ZIP file structure (symbol packages are ZIP files)
func extractSymbolPackageFilename(content []byte) (string, error) {
	// Read ZIP content to extract package information
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to read symbol package as ZIP: %w", err)
	}

	// Look for .pdb files or .nuspec files to determine package name
	var packageName, version string

	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, ".pdb") {
			// Extract from path like lib/net8.0/PackageName.pdb
			parts := strings.Split(file.Name, "/")
			if len(parts) > 0 {
				pdbName := parts[len(parts)-1]
				if strings.HasSuffix(pdbName, ".pdb") {
					packageName = strings.TrimSuffix(pdbName, ".pdb")
					break
				}
			}
		}
	}

	// If we can't extract from PDB, try to parse from any .nuspec file
	if packageName == "" {
		for _, file := range reader.File {
			if strings.HasSuffix(file.Name, ".nuspec") {
				// Parse nuspec file
				rc, err := file.Open()
				if err != nil {
					continue
				}
				defer rc.Close()

				nuspecContent, err := io.ReadAll(rc)
				if err != nil {
					continue
				}

				// Simple XML parsing to extract package ID and version
				packageName, version = parseSymbolNuspecContent(nuspecContent)
				if packageName != "" && version != "" {
					break
				}
			}
		}
	}

	// If still no package name, use a generic approach
	if packageName == "" {
		packageName = "unknown.package"
	}
	if version == "" {
		version = "1.0.0"
	}

	return fmt.Sprintf("%s.%s.snupkg", packageName, version), nil
}

// parseSymbolNuspecContent parses nuspec XML content to extract package ID and version
func parseSymbolNuspecContent(content []byte) (string, string) {
	// Simple regex-based parsing since we don't want to add XML dependency
	idRegex := regexp.MustCompile(`<id>([^<]+)</id>`)
	versionRegex := regexp.MustCompile(`<version>([^<]+)</version>`)

	var packageID, version string

	if match := idRegex.FindSubmatch(content); len(match) > 1 {
		packageID = string(match[1])
	}

	if match := versionRegex.FindSubmatch(content); len(match) > 1 {
		version = string(match[1])
	}

	return packageID, version
}

// isValidVersion checks if a string looks like a semantic version
func isValidVersion(version string) bool {
	// Simple check for version pattern: digits, dots, and optional pre-release/build metadata
	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	return versionRegex.MatchString(version)
}

// extractSymbolPackageInfo extracts package name and version from .snupkg file contents
func extractSymbolPackageInfo(fileContent []byte) (string, string, error) {
	// Create a byte reader for the zip content
	reader := bytes.NewReader(fileContent)
	zipReader, err := zip.NewReader(reader, int64(len(fileContent)))
	if err != nil {
		return "", "", fmt.Errorf("failed to read snupkg as zip: %w", err)
	}

	// Look for .nuspec file in the zip - symbol packages contain the same metadata
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, ".nuspec") {
			log.Info().Str("nuspec_file", file.Name).Msg("Found .nuspec file in symbol package")

			rc, err := file.Open()
			if err != nil {
				return "", "", fmt.Errorf("failed to open nuspec file: %w", err)
			}
			defer rc.Close()

			// Read and parse the .nuspec XML
			nuspecContent, err := io.ReadAll(rc)
			if err != nil {
				return "", "", fmt.Errorf("failed to read nuspec content: %w", err)
			}

			var nuspec NuSpec
			err = xml.Unmarshal(nuspecContent, &nuspec)
			if err != nil {
				return "", "", fmt.Errorf("failed to parse nuspec XML: %w", err)
			}

			if nuspec.Metadata.ID == "" || nuspec.Metadata.Version == "" {
				return "", "", fmt.Errorf("missing required metadata in nuspec file")
			}

			log.Info().
				Str("package_id", nuspec.Metadata.ID).
				Str("version", nuspec.Metadata.Version).
				Msg("Extracted symbol package metadata from .nuspec file")

			return nuspec.Metadata.ID, nuspec.Metadata.Version, nil
		}
	}

	return "", "", fmt.Errorf("no .nuspec file found in symbol package")
}
