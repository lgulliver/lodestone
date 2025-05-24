package nuget

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

// Registry implements the NuGet package registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new NuGet registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores a NuGet package
func (r *Registry) Upload(artifact *types.Artifact, content []byte) error {
	ctx := context.Background()

	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/octet-stream"); err != nil {
		return fmt.Errorf("failed to store NuGet package: %w", err)
	}

	artifact.ContentType = "application/octet-stream"
	return nil
}

// Download retrieves a NuGet package
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns NuGet packages matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a NuGet package
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid NuGet package
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty package content")
	}

	// Validate NuGet package ID format
	nugetIdRegex := regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9._]*$`)
	if !nugetIdRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid NuGet package ID format")
	}

	// Validate NuGet package version (SemVer 2.0)
	// This is a simplified validation
	semverRegex := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9\-\.]+)?(\+[a-zA-Z0-9\-\.]+)?$`)
	if !semverRegex.MatchString(artifact.Version) {
		return fmt.Errorf("invalid semantic version format")
	}

	// TODO: Validate .nupkg zip structure
	return nil
}

// GetMetadata extracts metadata from NuGet package
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from .nuspec file in the package
	return map[string]interface{}{
		"format":    "nuget",
		"type":      "package",
		"framework": ".NET",
	}, nil
}

// GenerateStoragePath creates the storage path for NuGet packages
func (r *Registry) GenerateStoragePath(name, version string) string {
	// NuGet follows: name/version/name.version.nupkg
	normalizedName := strings.ToLower(name)
	normalizedVersion := strings.ToLower(version)
	return fmt.Sprintf("nuget/%s/%s/%s.%s.nupkg", normalizedName, normalizedVersion, normalizedName, normalizedVersion)
}
