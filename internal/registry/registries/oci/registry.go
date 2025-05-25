package oci

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry implements the OCI/Docker container registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new OCI registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores an OCI artifact
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/octet-stream"); err != nil {
		return fmt.Errorf("failed to store OCI blob: %w", err)
	}

	artifact.ContentType = "application/octet-stream"
	return nil
}

// Download retrieves an OCI artifact
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns OCI artifacts matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an OCI artifact
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid OCI artifact
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty blob content")
	}

	// Validate image name format
	imageNameRegex := regexp.MustCompile(`^[a-z0-9]+(?:(?:\.|_|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:\.|_|-+)[a-z0-9]+)*)*$`)
	if !imageNameRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid OCI image name format")
	}

	// For tag validation
	if strings.HasPrefix(artifact.Version, "sha256:") {
		// This is a digest, validate SHA256 format
		digestRegex := regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
		if !digestRegex.MatchString(artifact.Version) {
			return fmt.Errorf("invalid digest format")
		}
	} else {
		// This is a tag, validate tag format
		tagRegex := regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}$`)
		if !tagRegex.MatchString(artifact.Version) {
			return fmt.Errorf("invalid tag format")
		}
	}

	return nil
}

// GetMetadata extracts metadata from OCI artifact
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// For OCI artifacts, we would typically extract metadata from the manifest
	// This is a simplified implementation
	return map[string]interface{}{
		"format": "oci",
		"type":   "container",
		"size":   len(content),
	}, nil
}

// GenerateStoragePath creates the storage path for OCI artifacts
func (r *Registry) GenerateStoragePath(name, version string) string {
	// OCI registry path structure depends on the specific artifact type
	if strings.HasPrefix(version, "sha256:") {
		// This is a blob digest
		digest := strings.TrimPrefix(version, "sha256:")
		return fmt.Sprintf("oci/%s/blobs/sha256/%s", name, digest)
	}

	// This is a tag, store manifest
	return fmt.Sprintf("oci/%s/manifests/%s", name, version)
}
