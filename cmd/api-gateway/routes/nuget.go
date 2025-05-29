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
	// Package content (flat container)
	nuget.GET("/v3-flatcontainer/:id/index.json", handleNuGetPackageVersions(registryService))
	nuget.GET("/v3-flatcontainer/:id/:version/:filename", middleware.OptionalAuthMiddleware(authService), handleNuGetDownload(registryService))

	// Package publish (requires authentication) - NuGet v2 API
	nuget.PUT("/v2/package", middleware.AuthMiddleware(authService), handleNuGetUpload(registryService))
	nuget.PUT("/v2/package/", middleware.AuthMiddleware(authService), handleNuGetUpload(registryService))
	nuget.DELETE("/v2/package/:id/:version", middleware.AuthMiddleware(authService), handleNuGetDelete(registryService))

	// Symbol package endpoints (requires authentication) - NuGet v2 API
	nuget.PUT("/v2/symbolpackage", middleware.AuthMiddleware(authService), handleNuGetSymbolUpload(registryService))
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

		// Log request details for debugging
		log.Info().
			Str("method", c.Request.Method).
			Str("content_type", c.GetHeader("Content-Type")).
			Int64("content_length", c.Request.ContentLength).
			Msg("Processing NuGet upload request")

		// Parse multipart form to check available fields
		err := c.Request.ParseMultipartForm(32 << 20) // 32MB max
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse multipart form")
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form", "details": err.Error()})
			return
		}

		// Log available form fields for debugging
		log.Info().Interface("form_fields", c.Request.MultipartForm.File).Msg("Available form fields")

		// Try different possible field names for the package file
		var file multipart.File
		var header *multipart.FileHeader
		for _, fieldName := range []string{"package", "file"} {
			file, header, err = c.Request.FormFile(fieldName)
			if err == nil {
				log.Info().Str("field_name", fieldName).Str("filename", header.Filename).Msg("Found file in form field")
				break
			}
		}

		// If no file found with known field names, try to get the first file
		if err != nil && c.Request.MultipartForm != nil && len(c.Request.MultipartForm.File) > 0 {
			for fieldName, files := range c.Request.MultipartForm.File {
				if len(files) > 0 {
					header = files[0]
					file, err = header.Open()
					log.Info().Str("field_name", fieldName).Str("filename", header.Filename).Msg("Using first available file")
					break
				}
			}
		}

		if err != nil {
			log.Error().Err(err).Msg("No package file found in upload")
			c.JSON(http.StatusBadRequest, gin.H{"error": "no package file found in upload", "details": err.Error()})
			return
		}
		defer file.Close()

		ctx := context.WithValue(c.Request.Context(), "registry", "nuget")
		ctx = context.WithValue(ctx, "user_id", user.ID)

		// Validate file extension
		filename := header.Filename
		log.Info().Str("filename", filename).Msg("Processing NuGet package filename")
		
		if !strings.HasSuffix(filename, ".nupkg") {
			log.Error().Str("filename", filename).Msg("Invalid package file extension")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package file extension"})
			return
		}

		// Read the entire file content to parse the .nupkg
		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read package file content")
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read package file"})
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

		// Create a new reader from the file content for upload
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
				"@type":        "Package",
				"commitId":     "00000000-0000-0000-0000-000000000000",
				"commitTimeStamp": artifact.CreatedAt,
				"catalogEntry": gin.H{
					"@id":         fmt.Sprintf("%s/v3/registration/%s/%s.json", 
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
					"@id":    fmt.Sprintf("%s/v3/registration/%s/index.json#page/1.0.0/%s",
						baseURL, strings.ToLower(packageID), artifacts[len(artifacts)-1].Version),
					"@type":  "catalog:CatalogPage",
					"count":  len(catalogEntries),
					"items":  catalogEntries,
					"lower":  artifacts[0].Version,
					"upper":  artifacts[len(artifacts)-1].Version,
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
