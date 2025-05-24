package registry

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lgulliver/lodestone/pkg/types"
)

// NPMRegistry implements the npm package registry
type NPMRegistry struct {
	service *Service
}

// NewNPMRegistry creates a new NPM registry handler
func NewNPMRegistry(service *Service) *NPMRegistry {
	return &NPMRegistry{service: service}
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
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns npm packages matching the filter
func (r *NPMRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an npm package
func (r *NPMRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid npm package
func (r *NPMRegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty package content")
	}
	
	// Check for gzip magic bytes
	if len(content) < 3 {
		return fmt.Errorf("invalid package format")
	}
	
	// Check for gzip signature
	if content[0] != 0x1f || content[1] != 0x8b {
		return fmt.Errorf("npm package must be gzipped tarball")
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
		"type":   "tgz",
	}
	
	// TODO: Extract package.json from tarball and parse metadata
	// This would involve decompressing the gzip and extracting package.json
	
	return metadata, nil
}
