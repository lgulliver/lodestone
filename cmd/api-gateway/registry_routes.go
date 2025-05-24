package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// setupRegistryRoutes configures all registry-specific routes
func setupRegistryRoutes(r *gin.Engine, registryService *registry.Service) {
	api := r.Group("/api/v1")
	
	// NuGet routes
	setupNuGetRoutes(api, registryService)
	
	// npm routes
	setupNPMRoutes(api, registryService)
	
	// Maven routes
	setupMavenRoutes(api, registryService)
	
	// Go module routes
	setupGoModuleRoutes(api, registryService)
	
	// Helm routes
	setupHelmRoutes(api, registryService)
	
	// OCI/Docker routes
	setupOCIRoutes(api, registryService)
	
	// Cargo routes
	setupCargoRoutes(api, registryService)
	
	// RubyGems routes
	setupRubyGemsRoutes(api, registryService)
	
	// OPA routes
	setupOPARoutes(api, registryService)
	
	// Generic artifact routes
	setupGenericRoutes(api, registryService)
}

// NuGet route setup
func setupNuGetRoutes(api *gin.RouterGroup, registryService *registry.Service) {
	nuget := api.Group("/nuget")
	
	// NuGet v3 API
	nuget.GET("/v3/index.json", handleNuGetServiceIndex())
	nuget.GET("/v3/flatcontainer/:id/index.json", handleNuGetPackageVersions(registryService))
	nuget.GET("/v3/flatcontainer/:id/:version/:filename", handleNuGetDownload(registryService))
	nuget.GET("/v3/query", handleNuGetSearch(registryService))
	
	// NuGet v2 API (for push)
	nuget.PUT("/", authenticateAPIKey(), handleNuGetPush(registryService))
	nuget.DELETE("/:id/:version", authenticateAPIKey(), handleNuGetDelete(registryService))
}

func handleNuGetServiceIndex() gin.HandlerFunc {
	return func(c *gin.Context) {
		baseURL := fmt.Sprintf("%s://%s%s", getScheme(c), c.Request.Host, "/api/v1/nuget")
		
		serviceIndex := map[string]interface{}{
			"version": "3.0.0",
			"resources": []map[string]interface{}{
				{
					"@id":      baseURL + "/v3/flatcontainer",
					"@type":    "PackageBaseAddress/3.0.0",
					"comment": "Base URL of where NuGet packages are stored",
				},
				{
					"@id":      baseURL + "/v3/query",
					"@type":    "SearchQueryService/3.0.0-rc",
					"comment": "Query endpoint of NuGet Search service",
				},
				{
					"@id":      baseURL + "/v3/registration",
					"@type":    "RegistrationsBaseUrl/3.0.0-beta",
					"comment": "Base URL of NuGet package registration",
				},
			},
		}

		c.JSON(http.StatusOK, serviceIndex)
	}
}

func handleNuGetPackageVersions(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		
		filter := &types.ArtifactFilter{
			Name:     packageID,
			Registry: "nuget",
		}
		
		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			logrus.WithError(err).Error("Failed to list NuGet package versions")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "Failed to retrieve package versions",
			})
			return
		}
		
		versions := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			versions = append(versions, artifact.Version)
		}
		
		response := map[string]interface{}{
			"versions": versions,
		}
		
		c.JSON(http.StatusOK, response)
	}
}

func handleNuGetDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		version := c.Param("version")
		filename := c.Param("filename")
		
		// Validate filename matches expected pattern
		expectedFilename := fmt.Sprintf("%s.%s.nupkg", strings.ToLower(packageID), strings.ToLower(version))
		if strings.ToLower(filename) != expectedFilename {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   "Package file not found",
			})
			return
		}
		
		_, content, err := registryService.Download(c.Request.Context(), "nuget", packageID, version)
		if err != nil {
			logrus.WithError(err).Error("Failed to download NuGet package")
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   "Package not found",
			})
			return
		}
		
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		
		// Stream the content to the response
		defer content.Close()
		c.Stream(func(w io.Writer) bool {
			_, err := io.Copy(w, content)
			return err == nil
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
		
		filter := &types.ArtifactFilter{
			Registry: "nuget",
			Name:     query, // Use query as name filter for now
			Offset:   skipInt,
			Limit:    takeInt,
		}
		
		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			logrus.WithError(err).Error("Failed to search NuGet packages")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "Search failed",
			})
			return
		}
		
		// Convert to NuGet search response format
		data := make([]map[string]interface{}, 0, len(artifacts))
		for _, artifact := range artifacts {
			description := ""
			if desc, ok := artifact.Metadata["description"]; ok {
				if descStr, ok := desc.(string); ok {
					description = descStr
				}
			}
			
			author := ""
			if auth, ok := artifact.Metadata["author"]; ok {
				if authStr, ok := auth.(string); ok {
					author = authStr
				}
			}
			
			tags := ""
			if tagData, ok := artifact.Metadata["tags"]; ok {
				if tagStr, ok := tagData.(string); ok {
					tags = tagStr
				}
			}
			
			data = append(data, map[string]interface{}{
				"id":          artifact.Name,
				"version":     artifact.Version,
				"description": description,
				"authors":     []string{author},
				"tags":        strings.Split(tags, ","),
				"totalDownloads": artifact.Downloads,
				"downloadUrl": fmt.Sprintf("/api/v1/nuget/v3/flatcontainer/%s/%s/%s.%s.nupkg",
					strings.ToLower(artifact.Name), strings.ToLower(artifact.Version),
					strings.ToLower(artifact.Name), strings.ToLower(artifact.Version)),
			})
		}
		
		response := map[string]interface{}{
			"totalHits": len(data),
			"data":      data,
		}
		
		c.JSON(http.StatusOK, response)
	}
}

func handleNuGetPush(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context (set by auth middleware)
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "Authentication required",
			})
			return
		}
		
		// Read the uploaded package
		file, header, err := c.Request.FormFile("package")
		if err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "No package file provided",
			})
			return
		}
		defer file.Close()
		
		content, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "Failed to read package content",
			})
			return
		}
		
		// Extract package metadata (this would be more sophisticated in practice)
		filename := header.Filename
		parts := strings.Split(strings.TrimSuffix(filename, ".nupkg"), ".")
		if len(parts) < 2 {
			c.JSON(http.StatusBadRequest, types.APIResponse{
				Success: false,
				Error:   "Invalid package filename format",
			})
			return
		}
		
		packageID := parts[0]
		version := strings.Join(parts[1:], ".")
		
		userObj := user.(*types.User)
		contentReader := bytes.NewReader(content)
		
		_, err = registryService.Upload(c.Request.Context(), "nuget", packageID, version, contentReader, userObj.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to upload NuGet package")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "Failed to upload package",
			})
			return
		}
		
		c.JSON(http.StatusCreated, types.APIResponse{
			Success: true,
			Message: "Package uploaded successfully",
		})
	}
}

func handleNuGetDelete(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		version := c.Param("version")
		
		// Check user permissions
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, types.APIResponse{
				Success: false,
				Error:   "Authentication required",
			})
			return
		}
		
		userObj := user.(*types.User)
		err := registryService.Delete(c.Request.Context(), "nuget", packageID, version, userObj.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to delete NuGet package")
			c.JSON(http.StatusInternalServerError, types.APIResponse{
				Success: false,
				Error:   "Failed to delete package",
			})
			return
		}
		
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Message: "Package deleted successfully",
		})
	}
}

// Helper function to determine scheme
func getScheme(c *gin.Context) string {
	if c.Request.TLS != nil {
		return "https"
	}
	if c.GetHeader("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}
