package registry

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/lgulliver/lodestone/pkg/types"
)

// GoRegistry implements the Go module registry
type GoRegistry struct {
	service *Service
}

// GoModInfo represents Go module information
type GoModInfo struct {
	Version string    `json:"Version"`
	Time    string    `json:"Time"`
	Origin  *GoOrigin `json:"Origin,omitempty"`
}

type GoOrigin struct {
	VCS    string `json:"VCS"`
	URL    string `json:"URL"`
	Subdir string `json:"Subdir,omitempty"`
	Ref    string `json:"Ref,omitempty"`
	Hash   string `json:"Hash,omitempty"`
}

// Upload stores a Go module
func (r *GoRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()

	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.Storage.Store(ctx, artifact.StoragePath, reader, "application/zip"); err != nil {
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
func (r *GoRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from go.mod file
	return map[string]interface{}{
		"format":     "go",
		"type":       "module",
		"go_version": "1.21", // Should be extracted from go.mod
	}, nil
}

// GenerateStoragePath creates the storage path for Go modules
func (r *GoRegistry) GenerateStoragePath(name, version string) string {
	// Go modules follow: module/@v/version.zip
	return fmt.Sprintf("go/%s/@v/%s.zip", name, version)
}

// CargoRegistry implements the Rust/Cargo package registry
type CargoRegistry struct {
	service *Service
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

// Upload stores a Cargo package
func (r *CargoRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()

	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.Storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store Cargo package: %w", err)
	}

	artifact.ContentType = "application/gzip"
	return nil
}

// Download retrieves a Cargo package
func (r *CargoRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns Cargo packages matching the filter
func (r *CargoRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a Cargo package
func (r *CargoRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid Cargo package
func (r *CargoRegistry) Validate(artifact *types.Artifact, content []byte) error {
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
func (r *CargoRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from Cargo.toml
	return map[string]interface{}{
		"format":       "cargo",
		"type":         "crate",
		"rust_version": "1.70.0", // Should be extracted
	}, nil
}

// GenerateStoragePath creates the storage path for Cargo packages
func (r *CargoRegistry) GenerateStoragePath(name, version string) string {
	// Cargo follows: crates/name/name-version.crate
	return fmt.Sprintf("cargo/crates/%s/%s-%s.crate", name, name, version)
}

// RubyGemsRegistry implements the RubyGems registry
type RubyGemsRegistry struct {
	service *Service
}

// Upload stores a RubyGem
func (r *RubyGemsRegistry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()

	// Store the content
	reader := bytes.NewReader(content)
	if err := r.service.Storage.Store(ctx, artifact.StoragePath, reader, "application/octet-stream"); err != nil {
		return fmt.Errorf("failed to store Ruby gem: %w", err)
	}

	artifact.ContentType = "application/octet-stream"
	return nil
}

// Download retrieves a RubyGem
func (r *RubyGemsRegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns RubyGems matching the filter
func (r *RubyGemsRegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a RubyGem
func (r *RubyGemsRegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid RubyGem
func (r *RubyGemsRegistry) Validate(artifact *types.Artifact, content []byte) error {
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
func (r *RubyGemsRegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from gemspec
	return map[string]interface{}{
		"format":       "rubygems",
		"type":         "gem",
		"ruby_version": "3.0.0", // Should be extracted
	}, nil
}

// GenerateStoragePath creates the storage path for RubyGems
func (r *RubyGemsRegistry) GenerateStoragePath(name, version string) string {
	// RubyGems follows: gems/name-version.gem
	return fmt.Sprintf("rubygems/gems/%s-%s.gem", name, version)
}

// OPARegistry implements the Open Policy Agent bundle registry
type OPARegistry struct {
	service *Service
}

// Upload stores an OPA bundle
func (r *OPARegistry) Upload(artifact *types.Artifact, content []byte) error {
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
func (r *OPARegistry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns OPA bundles matching the filter
func (r *OPARegistry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an OPA bundle
func (r *OPARegistry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid OPA bundle
func (r *OPARegistry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty bundle content")
	}

	// TODO: Validate bundle structure (should contain .rego files and optional manifest)
	return nil
}

// GetMetadata extracts metadata from OPA bundle
func (r *OPARegistry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from bundle manifest
	return map[string]interface{}{
		"format": "opa",
		"type":   "bundle",
	}, nil
}

// GenerateStoragePath creates the storage path for OPA bundles
func (r *OPARegistry) GenerateStoragePath(name, version string) string {
	// OPA bundles follow: bundles/name/version.tar.gz
	return fmt.Sprintf("opa/bundles/%s/%s.tar.gz", name, version)
}
