package opa

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry implements the Open Policy Agent bundle registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new OPA registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores an OPA bundle
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
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
