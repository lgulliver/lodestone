package maven

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

// Registry implements the Maven repository
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new Maven registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores a Maven artifact
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Determine content type based on file extension
	contentType := "application/java-archive"
	if strings.HasSuffix(artifact.Name, ".pom") {
		contentType = "application/xml"
	} else if strings.HasSuffix(artifact.Name, ".war") {
		contentType = "application/java-archive"
	} else if strings.HasSuffix(artifact.Name, ".aar") {
		contentType = "application/java-archive"
	}

	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, contentType); err != nil {
		return fmt.Errorf("failed to store Maven artifact: %w", err)
	}

	artifact.ContentType = contentType
	return nil
}

// Download retrieves a Maven artifact
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns Maven artifacts matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a Maven artifact
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid Maven artifact
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty artifact content")
	}

	// Validate Maven coordinates format (groupId:artifactId)
	parts := strings.Split(artifact.Name, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Maven coordinates format, expected groupId:artifactId")
	}

	groupId, artifactId := parts[0], parts[1]

	// Validate groupId format
	groupIdRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !groupIdRegex.MatchString(groupId) {
		return fmt.Errorf("invalid groupId format")
	}

	// Validate artifactId format
	artifactIdRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !artifactIdRegex.MatchString(artifactId) {
		return fmt.Errorf("invalid artifactId format")
	}

	// Validate Maven version format
	if artifact.Version == "" {
		return fmt.Errorf("invalid Maven version format: version cannot be empty")
	}

	// Maven version validation - allow alphanumeric, dots, hyphens, and common qualifiers
	versionRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)
	if !versionRegex.MatchString(artifact.Version) {
		return fmt.Errorf("invalid Maven version format")
	}

	// TODO: Validate JAR/WAR/AAR structure if applicable
	return nil
}

// GetMetadata extracts metadata from Maven artifact
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from POM file or JAR manifest
	return map[string]interface{}{
		"format":   "maven",
		"type":     "library",
		"language": "Java",
	}, nil
}

// GenerateStoragePath creates the storage path for Maven artifacts
func (r *Registry) GenerateStoragePath(name, version string) string {
	// Maven follows: groupId/artifactId/version/artifactId-version.jar
	parts := strings.Split(name, ":")
	if len(parts) != 2 {
		return fmt.Sprintf("maven/%s/%s", name, version)
	}

	groupId := strings.ReplaceAll(parts[0], ".", "/")
	artifactId := parts[1]

	return fmt.Sprintf("maven/%s/%s/%s/%s-%s.jar", groupId, artifactId, version, artifactId, version)
}
