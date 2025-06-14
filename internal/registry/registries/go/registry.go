package goregistry

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry implements the Go module registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new Go registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores a Go module
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/zip"); err != nil {
		return fmt.Errorf("failed to store Go module: %w", err)
	}

	artifact.ContentType = "application/zip"
	return nil
}

// Download retrieves a Go module
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns Go modules matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a Go module
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid Go module
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty module content")
	}

	// Validate module path format
	modulePathRegex := regexp.MustCompile(`^[a-z0-9.\-_~]+(/[a-z0-9.\-_~]+)*$`)
	if !modulePathRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid Go module path format")
	}

	// Validate semantic version
	semverRegex := regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-zA-Z0-9\-\.]+)?(\+[a-zA-Z0-9\-\.]+)?$`)
	if !semverRegex.MatchString(artifact.Version) {
		return fmt.Errorf("invalid semantic version format")
	}

	// TODO: Validate go.mod file exists in the ZIP
	return nil
}

// GetMetadata extracts metadata from Go module
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from go.mod file
	return map[string]interface{}{
		"format":     "go",
		"type":       "module",
		"go_version": "1.21", // Should be extracted from go.mod
	}, nil
}

// GenerateStoragePath creates the storage path for Go modules
func (r *Registry) GenerateStoragePath(name, version string) string {
	// Go modules follow: module/@v/version.zip
	return fmt.Sprintf("go/%s/@v/%s.zip", name, version)
}
