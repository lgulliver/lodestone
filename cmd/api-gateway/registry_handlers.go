package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// NuGet specific handlers
func handleNuGetServiceIndex() gin.HandlerFunc {
	return func(c *gin.Context) {
		// NuGet service index (v3 API)
		serviceIndex := map[string]interface{}{
			"version": "3.0.0",
			"resources": []map[string]interface{}{
				{
					"@id":      "/api/v1/nuget/v3/flatcontainer",
					"@type":    "PackageBaseAddress/3.0.0",
					"comment": "Base URL of where NuGet packages are stored",
				},
				{
					"@id":      "/api/v1/nuget/v3/query",
					"@type":    "SearchQueryService/3.0.0-rc",
					"comment": "Query endpoint of NuGet Search service",
				},
			},
		}

		c.JSON(http.StatusOK, serviceIndex)
	}
}

func handleNuGetUpload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// NuGet package upload (v2 API)
		// This is typically a PUT request with the package as form data
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Error:   "NuGet upload not yet implemented - use generic API",
		})
	}
}

func handleNuGetPackageVersions(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		
		// Return package versions in NuGet format
		versions := map[string]interface{}{
			"versions": []string{}, // TODO: Get actual versions from registry
		}

		c.JSON(http.StatusOK, versions)
	}
}

func handleNuGetDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageID := c.Param("id")
		version := c.Param("version")

		// Download package using generic registry service
		artifact, content, err := registryService.Download(c.Request.Context(), "nuget", packageID, version)
		if err != nil {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		defer content.Close()

		// Set NuGet-specific headers
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", string(rune(artifact.Size)))
		
		// Stream content
		c.DataFromReader(http.StatusOK, artifact.Size, "application/octet-stream", content, nil)
	}
}

// npm specific handlers
func handleNPMPackageInfo(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("name")
		
		// Return package metadata in npm format
		packageInfo := map[string]interface{}{
			"name":        packageName,
			"description": "Package hosted on Lodestone",
			"versions":    map[string]interface{}{}, // TODO: Get actual versions
			"time":        map[string]interface{}{}, // TODO: Get actual timestamps
		}

		c.JSON(http.StatusOK, packageInfo)
	}
}

func handleNPMPublish(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// npm publish implementation
		c.JSON(http.StatusNotImplemented, types.APIResponse{
			Success: false,
			Error:   "npm publish not yet implemented - use generic API",
		})
	}
}

func handleNPMDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("name")
		version := c.Param("version")

		// Download package using generic registry service
		artifact, content, err := registryService.Download(c.Request.Context(), "npm", packageName, version)
		if err != nil {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		defer content.Close()

		// Set npm-specific headers
		c.Header("Content-Type", "application/octet-stream")
		
		// Stream content
		c.DataFromReader(http.StatusOK, artifact.Size, "application/octet-stream", content, nil)
	}
}

// Go module proxy handlers
func handleGoModuleVersions(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		modulePath := c.Param("module")
		
		// Return available versions
		// TODO: Query actual versions from registry
		versions := "v1.0.0\nv1.1.0\n"
		
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, versions)
	}
}

func handleGoModuleInfo(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		modulePath := c.Param("module")
		version := c.Param("version")
		
		// Return version info in Go proxy format
		info := map[string]interface{}{
			"Version": version,
			"Time":    "2024-01-01T00:00:00Z", // TODO: Get actual timestamp
		}

		c.JSON(http.StatusOK, info)
	}
}

func handleGoModuleFile(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		modulePath := c.Param("module")
		version := c.Param("version")
		
		// Return go.mod file content
		// TODO: Extract go.mod from stored module
		goMod := "module " + modulePath + "\n\ngo 1.21\n"
		
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, goMod)
	}
}

func handleGoModuleDownload(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		modulePath := c.Param("module")
		version := c.Param("version")

		// Download module using generic registry service
		artifact, content, err := registryService.Download(c.Request.Context(), "go", modulePath, version)
		if err != nil {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		defer content.Close()

		// Set appropriate headers for Go modules
		c.Header("Content-Type", "application/zip")
		
		// Stream content
		c.DataFromReader(http.StatusOK, artifact.Size, "application/zip", content, nil)
	}
}
