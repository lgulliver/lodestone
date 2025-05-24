package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/lgulliver/lodestone/pkg/types"
)

// NPMRegistry implements the npm package registry
type NPMRegistry struct {
	service *Service
}

// Upload stores an npm package
func (r *NPMRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store npm package: %w", err)
	}
	
	artifact.ContentType = "application/gzip"
	return nil
}

// Download retrieves an npm package
func (r *NPMRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	// This would typically be called through the service layer
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns npm packages matching the filter
func (r *NPMRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	// This would typically be called through the service layer
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an npm package
func (r *NPMRegistry) Delete(name, version string) error {
	// This would typically be called through the service layer
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid npm package
func (r *NPMRegistry) Validate(artifact *types.Artifact, content []byte) error {
	// Basic validation for npm packages
	if len(content) == 0 {
		return fmt.Errorf("empty package content")
	}
	
	// Check if it's a gzipped tarball (npm packages are typically .tgz files)
	if len(content) < 2 {
		return fmt.Errorf("invalid package format")
	}
	
	// Check for gzip magic bytes
	if content[0] != 0x1f || content[1] != 0x8b {
		return fmt.Errorf("npm package must be a gzipped tarball")
	}
	
	// Validate package name
	if !isValidNPMPackageName(artifact.Name) {
		return fmt.Errorf("invalid npm package name: %s", artifact.Name)
	}
	
	return nil
}

// GetMetadata extracts metadata from npm package
func (r *NPMRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	metadata := map[string]interface{}{
		"format": "npm",
		"compressed": true,
	}
	
	// TODO: Extract package.json from tarball and parse metadata
	// This would involve decompressing the gzip and extracting package.json
	
	return metadata, nil
}

// NuGetRegistry implements the NuGet package registry
type NuGetRegistry struct {
	service *Service
}

// Upload stores a NuGet package
func (r *NuGetRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	
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
	
	// Check for ZIP magic bytes (NuGet packages are ZIP files)
	if len(content) < 4 {
		return fmt.Errorf("invalid package format")
	}
	
	// Check for ZIP signature
	if content[0] != 0x50 || content[1] != 0x4b {
		return fmt.Errorf("NuGet package must be a ZIP file")
	}
	
	// Validate package name
	if !isValidNuGetPackageName(artifact.Name) {
		return fmt.Errorf("invalid NuGet package name: %s", artifact.Name)
	}
	
	return nil
}

// GetMetadata extracts metadata from NuGet package
func (r *NuGetRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	metadata := map[string]interface{}{
		"format": "nuget",
		"type": "nupkg",
	}
	
	// TODO: Extract .nuspec file and parse metadata
	
	return metadata, nil
}

// GoRegistry implements the Go module registry
type GoRegistry struct {
	service *Service
}

// Upload stores a Go module
func (r *GoRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	
	reader := bytes.NewReader(content)
	if err := r.service.storage.Store(ctx, artifact.StoragePath, reader, "application/zip"); err != nil {
		return fmt.Errorf("failed to store Go module: %w", err)
	}
	
	artifact.ContentType = "application/zip"
	return nil
}

// Download retrieves a Go module
func (r *GoRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns Go modules matching the filter
func (r *GoRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a Go module
func (r *GoRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid Go module
func (r *GoRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty module content")
	}
	
	// Go modules should follow semver
	if !isValidSemver(artifact.Version) {
		return fmt.Errorf("invalid Go module version: %s", artifact.Version)
	}
	
	// Module name should be a valid Go module path
	if !isValidGoModulePath(artifact.Name) {
		return fmt.Errorf("invalid Go module name: %s", artifact.Name)
	}
	
	return nil
}

// GetMetadata extracts metadata from Go module
func (r *GoRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	metadata := map[string]interface{}{
		"format": "go",
		"type": "module",
	}
	
	// TODO: Parse go.mod file and extract dependencies
	
	return metadata, nil
}

// Utility functions for validation
func isValidNPMPackageName(name string) bool {
	// Basic npm package name validation
	if len(name) == 0 || len(name) > 214 {
		return false
	}
	
	// Must start with lowercase letter or @
	if name[0] != '@' && (name[0] < 'a' || name[0] > 'z') {
		return false
	}
	
	// Allow alphanumeric, hyphens, underscores, dots, and slashes (for scoped packages)
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || 
			 (char >= '0' && char <= '9') || 
			 char == '-' || char == '_' || char == '.' || char == '/' || char == '@') {
			return false
		}
	}
	
	return true
}

func isValidNuGetPackageName(name string) bool {
	// Basic NuGet package name validation
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	
	// Allow alphanumeric, dots, hyphens, underscores
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || 
			 (char >= 'A' && char <= 'Z') || 
			 (char >= '0' && char <= '9') || 
			 char == '.' || char == '-' || char == '_') {
			return false
		}
	}
	
	return true
}

func isValidGoModulePath(path string) bool {
	// Basic Go module path validation
	if len(path) == 0 {
		return false
	}
	
	// Should contain at least one dot (domain name)
	return strings.Contains(path, ".")
}

func isValidSemver(version string) bool {
	// Basic semver validation
	if len(version) == 0 {
		return false
	}
	
	// Should start with 'v' for Go modules
	if !strings.HasPrefix(version, "v") {
		return false
	}
	
	// Remove 'v' prefix and check basic format
	v := version[1:]
	parts := strings.Split(v, ".")
	return len(parts) >= 2 // At least major.minor
}
