package helm

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry implements the Helm chart repository
type Registry struct {
	storage storage.BlobStorage
	db      *common.Database
}

// New creates a new Helm registry handler
func New(storage storage.BlobStorage, db *common.Database) *Registry {
	return &Registry{
		storage: storage,
		db:      db,
	}
}

// Upload stores a Helm chart
func (r *Registry) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	// Store the content
	reader := bytes.NewReader(content)
	if err := r.storage.Store(ctx, artifact.StoragePath, reader, "application/gzip"); err != nil {
		return fmt.Errorf("failed to store Helm chart: %w", err)
	}

	artifact.ContentType = "application/gzip"
	return nil
}

// Download retrieves a Helm chart
func (r *Registry) Download(name, version string) (*types.Artifact, []byte, error) {
	return nil, nil, fmt.Errorf("use service.Download instead")
}

// List returns Helm charts matching the filter
func (r *Registry) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	return nil, fmt.Errorf("use service.List instead")
}

// Delete removes a Helm chart
func (r *Registry) Delete(name, version string) error {
	return fmt.Errorf("use service.Delete instead")
}

// Validate checks if the artifact is a valid Helm chart
func (r *Registry) Validate(artifact *types.Artifact, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty chart content")
	}

	// Validate chart name format
	chartNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !chartNameRegex.MatchString(artifact.Name) {
		return fmt.Errorf("invalid Helm chart name format")
	}

	// TODO: Validate chart structure (Chart.yaml, templates/, etc.)
	return nil
}

// GetMetadata extracts metadata from Helm chart
func (r *Registry) GetMetadata(content []byte) (map[string]interface{}, error) {
	// TODO: Extract metadata from Chart.yaml
	return map[string]interface{}{
		"format": "helm",
		"type":   "chart",
	}, nil
}

// GenerateStoragePath creates the storage path for Helm charts
func (r *Registry) GenerateStoragePath(name, version string) string {
	// Helm charts follow: charts/name-version.tgz
	return fmt.Sprintf("helm/charts/%s-%s.tgz", name, version)
}
