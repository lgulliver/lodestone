package rubygems

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

// Registry implements the RubyGems registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new RubyGems registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores a RubyGem
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/octet-stream"); err != nil {
		return fmt.Errorf("failed to store Ruby gem: %w", err)
	}

	artifact.ContentType = "application/octet-stream"
	return nil
}

// Download retrieves a RubyGem
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns RubyGems matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a RubyGem
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid RubyGem
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty gem content")
	}

	// Check file extension
	if !strings.HasSuffix(strings.ToLower(artifact.Name), ".gem") {
		return fmt.Errorf("invalid gem file extension")
	}

	// Validate gem name format
	gemNameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	baseName := strings.TrimSuffix(artifact.Name, ".gem")
	if !gemNameRegex.MatchString(baseName) {
		return fmt.Errorf("invalid gem name format")
	}

	// TODO: Validate gem file structure
	return nil
}

// GetMetadata extracts metadata from RubyGem
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from gemspec
	return map[string]interface{}{
		"format":       "rubygems",
		"type":         "gem",
		"ruby_version": "3.0.0", // Should be extracted
	}, nil
}

// GenerateStoragePath creates the storage path for RubyGems
func (r *Registry) GenerateStoragePath(name, version string) string {
	// RubyGems follows: gems/name-version.gem
	return fmt.Sprintf("rubygems/gems/%s-%s.gem", name, version)
}
