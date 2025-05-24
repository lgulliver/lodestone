package opa_registry

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry implements the Open Policy Agent bundle registry
type Registry struct {
	service *registry.Service
}

// New creates a new OPA registry handler
func New(service *registry.Service) *Registry {
	return &Registry{
		service: service,
	}
}

// Upload stores an OPA bundle
func (r *Registry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()
	
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.Storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store OPA bundle: %w", err)
	}
	
	artifact.ContentType = "application/gzip"
	return nil
}

// Download retrieves an OPA bundle
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns OPA bundles matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an OPA bundle
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid OPA bundle
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty bundle content")
	}

	// TODO: Validate bundle structure (should contain .rego files and optional manifest)
	return nil
}

// GetMetadata extracts metadata from OPA bundle
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from bundle manifest
	return map[string]interface{}{
		"format": "opa",
		"type":   "bundle",
	}, nil
}

// GenerateStoragePath creates the storage path for OPA bundles
func (r *Registry) GenerateStoragePath(name, version string) string {
	// OPA bundles follow: bundles/name/version.tar.gz
	return fmt.Sprintf("opa/bundles/%s/%s.tar.gz", name, version)
}
