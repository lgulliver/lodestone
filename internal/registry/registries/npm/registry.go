package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
)

// Registry implements the npm package registry
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// PackageManifest represents package.json structure
type PackageManifest struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description,omitempty"`
	Author          interface{}       `json:"author,omitempty"` // Can be string or object
	License         string            `json:"license,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
	Keywords        []string          `json:"keywords,omitempty"`
	Repository      interface{}       `json:"repository,omitempty"` // Can be string or object
}

// New creates a new npm registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores an npm package
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	log.Info().
		Str("package", artifact.Name).
		Str("version", artifact.Version).
		Str("storage_path", artifact.StoragePath).
		Int("content_size", len(content)).
		Msg("Starting NPM package storage")

	// Store the content
	reader := bytes.NewReader(content)

	log.Debug().
		Str("storage_path", artifact.StoragePath).
		Int("content_size", len(content)).
		Msg("Calling storage.Store for NPM package")

	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/octet-stream"); err != nil {
		log.Error().
			Err(err).
			Str("package", artifact.Name).
			Str("version", artifact.Version).
			Str("storage_path", artifact.StoragePath).
			Msg("Failed to store NPM package to storage")
		return fmt.Errorf("failed to store npm package: %w", err)
	}

	log.Info().
		Str("package", artifact.Name).
		Str("version", artifact.Version).
		Str("storage_path", artifact.StoragePath).
		Msg("NPM package stored successfully to storage")

	artifact.ContentType = "application/octet-stream"
	return nil
}

// Download retrieves an npm package
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns npm packages matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes an npm package
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid npm package
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty package content")
	}

	// Validate npm package name format
	// Allow scoped packages (@org/name)
	npmNameRegex := regexp.MustCompile(`^(@[a-z0-9-~][a-z0-9-._~]*/)?[a-z0-9-~][a-z0-9-._~]*$`)
	if !npmNameRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid npm package name format")
	}

	// Extract and validate package.json from the tarball
	packageJSON, err := extractPackageJSONFromTarball(content)
	if err != nil {
		return fmt.Errorf("failed to extract package.json: %w", err)
	}

	// Validate name and version match
	if packageJSON.Name != "" && packageJSON.Name != artifact.Name {
		return fmt.Errorf("package name mismatch: %s vs %s", packageJSON.Name, artifact.Name)
	}

	if packageJSON.Version != "" && packageJSON.Version != artifact.Version {
		return fmt.Errorf("package version mismatch: %s vs %s", packageJSON.Version, artifact.Version)
	}

	return nil
}

// GetMetadata extracts metadata from npm package
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	metadata := map[string]interface{}{
		"format": "npm",
		"type":   "package",
	}

	// Extract package.json from the tarball
	packageJSON, err := extractPackageJSONFromTarball(content)
	if err != nil {
		// If we can't extract package.json, return basic metadata
		return metadata, nil
	}

	// Add extracted metadata
	if packageJSON.Description != "" {
		metadata["description"] = packageJSON.Description
	}
	if packageJSON.License != "" {
		metadata["license"] = packageJSON.License
	}
	if len(packageJSON.Keywords) > 0 {
		metadata["keywords"] = packageJSON.Keywords
	}
	if len(packageJSON.Dependencies) > 0 {
		metadata["dependencies"] = packageJSON.Dependencies
	}
	if len(packageJSON.DevDependencies) > 0 {
		metadata["devDependencies"] = packageJSON.DevDependencies
	}

	// Handle author field (can be string or object)
	if packageJSON.Author != nil {
		metadata["author"] = packageJSON.Author
	}

	// Handle repository field (can be string or object)
	if packageJSON.Repository != nil {
		metadata["repository"] = packageJSON.Repository
	}

	return metadata, nil
}

// extractPackageJSONFromTarball extracts and parses package.json from an npm tarball
func extractPackageJSONFromTarball(tarballData []byte) (*PackageManifest, error) {
	// Create a gzip reader
	gzipReader, err := gzip.NewReader(bytes.NewReader(tarballData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Look for package.json in the tarball
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Look for package.json file (could be in package/ directory)
		if strings.HasSuffix(header.Name, "package.json") {
			// Read the package.json content
			packageJSONBytes, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read package.json: %w", err)
			}

			// Parse package.json
			var packageJSON PackageManifest
			if err := json.Unmarshal(packageJSONBytes, &packageJSON); err != nil {
				return nil, fmt.Errorf("failed to parse package.json: %w", err)
			}

			return &packageJSON, nil
		}
	}

	return nil, fmt.Errorf("package.json not found in tarball")
}

// GenerateStoragePath creates the storage path for npm packages
func (r *Registry) GenerateStoragePath(name, version string) string {
	// If this is a scoped package, handle the path differently
	if strings.HasPrefix(name, "@") {
		// Convert @scope/name to @scope%2fname format
		encodedName := strings.ReplaceAll(name, "/", "%2f")
		return fmt.Sprintf("npm/%s/%s.tgz", encodedName, version)
	}

	return fmt.Sprintf("npm/%s/%s.tgz", name, version)
}
