package registry

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"

	"github.com/lgulliver/lodestone/pkg/types"
)

// NuGetRegistry implements the NuGet package registry
type NuGetRegistry struct {
	service *Service
}

// NuSpec represents the structure of a NuGet package specification
type NuSpec struct {
	XMLName  xml.Name `xml:"package"`
	Metadata struct {
		ID          string `xml:"id"`
		Version     string `xml:"version"`
		Title       string `xml:"title"`
		Authors     string `xml:"authors"`
		Description string `xml:"description"`
		Tags        string `xml:"tags"`
	} `xml:"metadata"`
}

// Upload stores a NuGet package
func (r *NuGetRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/zip"); err != nil {
		return fmt.Errorf("failed to store NuGet package: %w", err)
	}
	
	artifact.ContentType = "application/zip"
	return nil
}

// Download retrieves a NuGet package
func (r *NuGetRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns NuGet packages matching the filter
func (r *NuGetRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a NuGet package
func (r *NuGetRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid NuGet package
func (r *NuGetRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty package content")
	}

	// Check file extension
	if !strings.HasSuffix(strings.ToLower(artifact.Name), ".nupkg") {
		return fmt.Errorf("invalid NuGet package file extension")
	}

	// Validate package ID format
	packageIDRegex := regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	if !packageIDRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid package ID format")
	}

	// TODO: Add ZIP validation and check for .nuspec file inside
	return nil
}

// GetMetadata extracts metadata from NuGet package
func (r *NuGetRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from .nuspec file within the ZIP
	return map[string]interface{}{
		"format":      "nuget",
		"type":        "package",
		"framework":   "netstandard2.0", // Default, should be extracted
		"description": "",
		"authors":     "",
		"tags":        []string{},
	}, nil
}

// GenerateStoragePath creates the storage path for NuGet packages
func (r *NuGetRegistry) GenerateStoragePath(name, version string) string {
	// NuGet follows: packageid/version/packageid.version.nupkg
	lowerName := strings.ToLower(name)
	lowerVersion := strings.ToLower(version)
	return fmt.Sprintf("nuget/%s/%s/%s.%s.nupkg", lowerName, lowerVersion, lowerName, lowerVersion)
}

// GetServiceIndexResponse returns NuGet service index
func (r *NuGetRegistry) GetServiceIndexResponse(baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"version": "3.0.0",
		"resources": []map[string]interface{}{
			{
				"@id":   baseURL + "/nuget/v3/query",
				"@type": "SearchQueryService/3.0.0-beta",
			},
			{
				"@id":   baseURL + "/nuget/v3/registration",
				"@type": "RegistrationsBaseUrl/3.0.0-beta",
			},
			{
				"@id":   baseURL + "/nuget/packages",
				"@type": "PackageBaseAddress/3.0.0",
			},
		},
	}
}
