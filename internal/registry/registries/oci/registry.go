package oci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// Registry implements the OCI/Docker container registry
type Registry struct {
	storage        storage.BlobStorage
	db             *common.Database
	sessionManager *SessionManager
}

// New creates a new OCI registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage:        storage,
		db:             db,
		sessionManager: NewSessionManager(storage),
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

// StartBlobUpload starts a new blob upload session
func (r *Registry) StartBlobUpload(ctx context.Context, repository, userID string) (*UploadSession, error) {
	return r.sessionManager.StartUpload(ctx, repository, userID)
}

// AppendBlobChunk appends data to an existing upload session
func (r *Registry) AppendBlobChunk(ctx context.Context, sessionID string, data io.Reader, contentRange string) (*UploadSession, error) {
	return r.sessionManager.AppendChunk(ctx, sessionID, data, contentRange)
}

// CompleteBlobUpload completes a blob upload with digest verification
func (r *Registry) CompleteBlobUpload(ctx context.Context, sessionID, expectedDigest string) (*UploadSession, string, error) {
	return r.sessionManager.CompleteUpload(ctx, sessionID, expectedDigest)
}

// CancelBlobUpload cancels an active upload session
func (r *Registry) CancelBlobUpload(ctx context.Context, sessionID string) error {
	return r.sessionManager.CancelUpload(ctx, sessionID)
}

// GetBlobUploadStatus returns the status of an upload session
func (r *Registry) GetBlobUploadStatus(sessionID string) (*UploadSession, error) {
	return r.sessionManager.GetUploadStatus(sessionID)
}

// BlobExists checks if a blob exists in the registry
func (r *Registry) BlobExists(ctx context.Context, repository, digest string) (bool, int64, error) {
	path := fmt.Sprintf("oci/%s/blobs/%s", repository, digest)
	exists, err := r.storage.Exists(ctx, path)
	if err != nil || !exists {
		return false, 0, err
	}

	size, err := r.storage.GetSize(ctx, path)
	return true, size, err
}

// GetBlob retrieves a blob from storage
func (r *Registry) GetBlob(ctx context.Context, repository, digest string) (io.ReadCloser, int64, error) {
	path := fmt.Sprintf("oci/%s/blobs/%s", repository, digest)

	// Check if blob exists
	exists, err := r.storage.Exists(ctx, path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check blob existence: %w", err)
	}
	if !exists {
		return nil, 0, fmt.Errorf("blob not found")
	}

	// Get blob size
	size, err := r.storage.GetSize(ctx, path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get blob size: %w", err)
	}

	// Retrieve blob content
	reader, err := r.storage.Retrieve(ctx, path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve blob: %w", err)
	}

	return reader, size, nil
}

// DeleteBlob removes a blob from storage
func (r *Registry) DeleteBlob(ctx context.Context, repository, digest string) error {
	path := fmt.Sprintf("oci/%s/blobs/%s", repository, digest)
	return r.storage.Delete(ctx, path)
}

// ManifestExists checks if a manifest exists and returns its digest, size, and media type
func (r *Registry) ManifestExists(ctx context.Context, repository, reference string) (bool, string, int64, string, error) {
	path := fmt.Sprintf("oci/%s/manifests/%s", repository, reference)

	exists, err := r.storage.Exists(ctx, path)
	if err != nil {
		return false, "", 0, "", fmt.Errorf("failed to check manifest existence: %w", err)
	}

	if !exists {
		return false, "", 0, "", nil
	}

	// Get size
	size, err := r.storage.GetSize(ctx, path)
	if err != nil {
		return false, "", 0, "", fmt.Errorf("failed to get manifest size: %w", err)
	}

	// Read manifest to get digest and media type
	reader, err := r.storage.Retrieve(ctx, path)
	if err != nil {
		return false, "", 0, "", fmt.Errorf("failed to read manifest: %w", err)
	}
	defer reader.Close()

	manifestContent, err := io.ReadAll(reader)
	if err != nil {
		return false, "", 0, "", fmt.Errorf("failed to read manifest content: %w", err)
	}

	// Calculate digest
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(manifestContent))

	// Parse manifest to get media type
	var manifestObj map[string]interface{}
	mediaType := "application/vnd.docker.distribution.manifest.v2+json" // default
	if err := json.Unmarshal(manifestContent, &manifestObj); err == nil {
		if mt, ok := manifestObj["mediaType"].(string); ok {
			mediaType = mt
		}
	}

	return true, digest, size, mediaType, nil
}

// GetManifest retrieves a manifest from storage
func (r *Registry) GetManifest(ctx context.Context, repository, reference string) (io.ReadCloser, string, int64, error) {
	path := fmt.Sprintf("oci/%s/manifests/%s", repository, reference)

	// Check if manifest exists first
	exists, digest, size, _, err := r.ManifestExists(ctx, repository, reference)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to check manifest existence: %w", err)
	}
	if !exists {
		return nil, "", 0, fmt.Errorf("manifest not found")
	}

	// Retrieve manifest content
	reader, err := r.storage.Retrieve(ctx, path)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to retrieve manifest: %w", err)
	}

	return reader, digest, size, nil
}

// PutManifest stores a manifest
func (r *Registry) PutManifest(ctx context.Context, repository, reference string, content io.Reader, contentType string) (string, error) {
	// Read all content to calculate digest
	data, err := io.ReadAll(content)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest content: %w", err)
	}

	// Validate that it's a valid JSON manifest
	if contentType == "application/vnd.docker.distribution.manifest.v2+json" ||
		contentType == "application/vnd.oci.image.manifest.v1+json" {
		var manifest map[string]interface{}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return "", fmt.Errorf("invalid manifest JSON: %w", err)
		}
	}

	// Calculate digest
	hasher := sha256.New()
	hasher.Write(data)
	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	// Store manifest
	path := fmt.Sprintf("oci/%s/manifests/%s", repository, reference)
	err = r.storage.Store(ctx, path, bytes.NewReader(data), contentType)
	if err != nil {
		return "", fmt.Errorf("failed to store manifest: %w", err)
	}

	// Also store by digest for direct access
	digestPath := fmt.Sprintf("oci/%s/manifests/%s", repository, digest)
	err = r.storage.Store(ctx, digestPath, bytes.NewReader(data), contentType)
	if err != nil {
		// Log error but don't fail the operation
		log.Warn().Err(err).Str("path", digestPath).Msg("Failed to store manifest by digest")
	}

	log.Info().
		Str("repository", repository).
		Str("reference", reference).
		Str("digest", digest).
		Str("content_type", contentType).
		Int("size", len(data)).
		Msg("Stored manifest")

	return digest, nil
}

// DeleteManifest removes a manifest from storage
func (r *Registry) DeleteManifest(ctx context.Context, repository, reference string) error {
	// Get digest before deletion for cleanup
	_, digest, _, _, err := r.ManifestExists(ctx, repository, reference)
	if err != nil {
		return err
	}

	// Delete manifest by reference
	path := fmt.Sprintf("oci/%s/manifests/%s", repository, reference)
	err = r.storage.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete manifest: %w", err)
	}

	// Also try to delete by digest (ignore errors)
	if digest != "" {
		digestPath := fmt.Sprintf("oci/%s/manifests/%s", repository, digest)
		r.storage.Delete(ctx, digestPath)
	}

	return nil
}

// ListTags returns all tags for a repository
func (r *Registry) ListTags(ctx context.Context, repository string) ([]string, error) {
	prefix := fmt.Sprintf("oci/%s/manifests/", repository)
	paths, err := r.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list manifests: %w", err)
	}

	var tags []string
	for _, path := range paths {
		// Extract tag from path: oci/repo/manifests/tag -> tag
		parts := strings.Split(path, "/")
		if len(parts) >= 4 {
			tag := parts[len(parts)-1]
			// Skip digest-based manifests (they start with sha256:)
			if !strings.HasPrefix(tag, "sha256:") {
				tags = append(tags, tag)
			}
		}
	}

	return tags, nil
}

// ListRepositories returns all repositories
func (r *Registry) ListRepositories(ctx context.Context) ([]string, error) {
	paths, err := r.storage.List(ctx, "oci/")
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	repoSet := make(map[string]bool)
	for _, path := range paths {
		// Extract repository from path: oci/repo/... -> repo
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			repo := parts[1]
			repoSet[repo] = true
		}
	}

	var repositories []string
	for repo := range repoSet {
		repositories = append(repositories, repo)
	}

	return repositories, nil
}
