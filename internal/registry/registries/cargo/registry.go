package cargo

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry implements the Rust/Cargo package registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// CargoManifest represents Cargo.toml structure
type CargoManifest struct {
	Package struct {
		Name        string   `toml:"name"`
		Version     string   `toml:"version"`
		Authors     []string `toml:"authors"`
		Description string   `toml:"description"`
		Keywords    []string `toml:"keywords"`
		Categories  []string `toml:"categories"`
		License     string   `toml:"license"`
		Repository  string   `toml:"repository"`
	} `toml:"package"`
}

// New creates a new Cargo registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores a Cargo package
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store Cargo package: %w", err)
	}

	artifact.ContentType = "application/gzip"
	return nil
}

// Download retrieves a Cargo package
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns Cargo packages matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a Cargo package
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid Cargo package
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty crate content")
	}

	// Validate crate name format
	crateNameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !crateNameRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid crate name format")
	}

	// TODO: Validate .crate file structure and Cargo.toml
	return nil
}

// GetMetadata extracts metadata from Cargo package
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from Cargo.toml
	return map[string]interface{}{
		"format":       "cargo",
		"type":         "crate",
		"rust_version": "1.70.0", // Should be extracted
	}, nil
}

// GenerateStoragePath creates the storage path for Cargo packages
func (r *Registry) GenerateStoragePath(name, version string) string {
	// Cargo follows: crates/name/name-version.crate
	return fmt.Sprintf("cargo/crates/%s/%s-%s.crate", name, name, version)
}
