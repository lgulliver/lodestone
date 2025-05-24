package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
)

// npm route setup
func setupNPMRoutes(api *gin.RouterGroup, registryService *registry.Service) {
	npm := api.Group("/npm")
	
	// npm registry API
	npm.GET("/:package", handleNPMPackageMetadata(registryService))
	npm.GET("/:package/:version", handleNPMPackageVersion(registryService))
	npm.GET("/-/package/:package/dist-tags", handleNPMDistTags(registryService))
	npm.PUT("/:package", authenticateAPIKey(), handleNPMPublish(registryService))
	npm.DELETE("/:package/-/:filename/-rev/:rev", authenticateAPIKey(), handleNPMUnpublish(registryService))
	
	// npm search
	npm.GET("/-/v1/search", handleNPMSearch(registryService))
	
	// npm login/whoami
	npm.PUT("/-/user/org.couchdb.user:*", handleNPMAddUser())
	npm.GET("/-/whoami", authenticateAPIKey(), handleNPMWhoami())
}

func handleNPMPackageMetadata(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("package")
		
		// Handle scoped packages
		if strings.HasPrefix(packageName, "@") {
			scope := c.Param("package")
			pkg := c.Param("*") // This would need proper routing setup
			packageName = scope + "/" + pkg
		}
		
		filter := &types.ArtifactFilter{
			Name:     packageName,
			Registry: "npm",
		}
		
		artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			logrus.WithError(err).Error("Failed to get npm package metadata")
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Package not found",
			})
			return
		}
		
		if len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Package not found",
			})
			return
		}
		
		// Build npm package metadata response
		versions := make(map[string]interface{})
		distTags := map[string]string{
			"latest": artifacts[len(artifacts)-1].Version, // Assume last is latest
		}
		
		for _, artifact := range artifacts {
			tarballURL := fmt.Sprintf("%s://%s/api/v1/npm/%s/-/%s-%s.tgz",
				getScheme(c), c.Request.Host, 
				url.PathEscape(packageName), 
				packageName, artifact.Version)
					// Build version metadata
		description := ""
		author := ""
		if artifact.Metadata != nil {
			if desc, ok := artifact.Metadata["description"].(string); ok {
				description = desc
			}
			if auth, ok := artifact.Metadata["author"].(string); ok {
				author = auth
			}
		}
		
		versions[artifact.Version] = map[string]interface{}{
			"name":        artifact.Name,
			"version":     artifact.Version,
			"description": description,
			"main":        "index.js",
			"author":      author,
			"license":     "MIT", // Should be extracted from package.json
			"dist": map[string]interface{}{
				"tarball": tarballURL,
				"shasum":  artifact.SHA256,
			},
		}
		}
		
		response := map[string]interface{}{
			"_id":       packageName,
			"name":      packageName,
			"versions":  versions,
			"dist-tags": distTags,
			"time": map[string]interface{}{
				"created":  artifacts[0].CreatedAt,
				"modified": artifacts[len(artifacts)-1].UpdatedAt,
			},
		}
		
		c.JSON(http.StatusOK, response)
	}
}

func handleNPMPackageVersion(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("package")
		version := c.Param("version")
		
	artifact, content, err := registryService.Download(c.Request.Context(), "npm", packageName, version)
	if err != nil {
		logrus.WithError(err).Error("Failed to download npm package")
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Package version not found",
		})
		return
	}
	defer content.Close()
	
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(artifact.Size, 10))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.tgz", packageName, version))
	
	// Stream the content
	io.Copy(c.Writer, content)
	}
}

func handleNPMDistTags(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("package")
		
		filter := &types.ArtifactFilter{		Name:     packageName,
		Registry: "npm",
	}
	
	artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil || len(artifacts) == 0 {
			c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "Package not found",
			})
			return
		}
		
		// Return dist-tags (simplified - just latest)
		distTags := map[string]string{
			"latest": artifacts[len(artifacts)-1].Version,
		}
		
		c.JSON(http.StatusOK, distTags)
	}
}

func handleNPMPublish(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error": "Authentication required",
			})
			return
		}
		
		// Read the package data
		var packageData map[string]interface{}
		if err := c.ShouldBindJSON(&packageData); err != nil {
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Invalid package data",
			})
			return
		}
		
		// Extract package metadata
		name, ok := packageData["name"].(string)
		if !ok {
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Package name is required",
			})
			return
		}
		
		version, ok := packageData["version"].(string)
		if !ok {
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Package version is required",
			})
			return
		}
		
		// Extract tarball data from attachments
		attachments, ok := packageData["_attachments"].(map[string]interface{})
		if !ok || len(attachments) == 0 {
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Package tarball is required",
			})
			return
		}
		
		// Get the first (and should be only) attachment
		var tarballData string
		for _, attachment := range attachments {
			if attachmentMap, ok := attachment.(map[string]interface{}); ok {
				if data, ok := attachmentMap["data"].(string); ok {
					tarballData = data
					break
				}
			}
		}
		
		if tarballData == "" {
			c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Invalid tarball data",
			})
			return
		}
		
	// Decode base64 tarball data
	content, err := utils.DecodeBase64(tarballData)
	if err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid tarball encoding",
		})
		return
	}
	
	// Create a reader from the content
	contentReader := bytes.NewReader(content)
	
	// Upload using the registry service
	userObj := c.MustGet("user").(*types.User)
	artifact, err := registryService.Upload(c.Request.Context(), "npm", name, version, contentReader, userObj.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed to upload npm package")
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to upload package",
		})
		return
	}
	
	c.JSON(http.StatusCreated, map[string]interface{}{
		"ok": true,
		"id": artifact.ID,
	})
	}
}

func handleNPMUnpublish(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		packageName := c.Param("package")
		filename := c.Param("filename")
		
		// Extract version from filename
		// filename format: package-version.tgz
		version := strings.TrimSuffix(filename, ".tgz")
		version = strings.TrimPrefix(version, packageName+"-")
		
		user := c.MustGet("user").(*types.User)
		err := registryService.Delete(c.Request.Context(), "npm", packageName, version, user.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to delete npm package")
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to delete package",
			})
			return
		}
		
		c.JSON(http.StatusOK, map[string]interface{}{
			"ok": true,
		})
	}
}

func handleNPMSearch(registryService *registry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		text := c.Query("text")
		size := c.DefaultQuery("size", "20")
		from := c.DefaultQuery("from", "0")
		
		sizeInt, _ := strconv.Atoi(size)
		fromInt, _ := strconv.Atoi(from)
		
	filter := &types.ArtifactFilter{
		Registry: "npm",
		Name:     text,  // Use Name instead of Query
		Limit:    sizeInt,
		Offset:   fromInt,
	}
	
	artifacts, _, err := registryService.List(c.Request.Context(), filter)
		if err != nil {
			logrus.WithError(err).Error("Failed to search npm packages")
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Search failed",
			})
			return
		}
		
		// Group by package name and get latest version
		packageMap := make(map[string]*types.Artifact)
		for _, artifact := range artifacts {
			if existing, exists := packageMap[artifact.Name]; !exists || artifact.CreatedAt.After(existing.CreatedAt) {
				packageMap[artifact.Name] = artifact
			}
		}
		
		objects := make([]map[string]interface{}, 0, len(packageMap))
		for _, artifact := range packageMap {
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
			
			objects = append(objects, map[string]interface{}{
				"package": map[string]interface{}{
					"name":        artifact.Name,
					"version":     artifact.Version,
					"description": description,
					"author": map[string]interface{}{
						"name": author,
					},
					"date": artifact.CreatedAt,
				},
				"score": map[string]interface{}{
					"final":   0.8,
					"detail": map[string]interface{}{
						"quality":     0.8,
						"popularity":  0.7,
						"maintenance": 0.9,
					},
				},
			})
		}
		
		response := map[string]interface{}{
			"total":   len(objects),
			"time":    "1ms",
			"objects": objects,
		}
		
		c.JSON(http.StatusOK, response)
	}
}

func handleNPMAddUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		// npm adduser - for now just return success
		// In a real implementation, this would create a user account
		c.JSON(http.StatusOK, map[string]interface{}{
			"ok": true,
			"id": "org.couchdb.user:testuser",
			"rev": "1-123456789",
		})
	}
}

func handleNPMWhoami() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error": "Authentication required",
			})
			return
		}
		
		c.JSON(http.StatusOK, map[string]interface{}{
			"username": user.(*types.User).Username,
		})
	}
}
