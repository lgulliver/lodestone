package registry

import "github.com/lgulliver/lodestone/pkg/types"

// Handler defines the interface that all registry implementations must implement
type Handler interface {
	// Upload stores an artifact in the registry
	Upload(artifact *types.Artifact, content []byte) error

	// Download retrieves an artifact from the registry
	// Note: This method is deprecated - use service.Download instead
	Download(name, version string) (*types.Artifact, []byte, error)

	// List returns artifacts matching the filter
	// Note: This method is deprecated - use service.List instead
	List(filter *types.ArtifactFilter) ([]*types.Artifact, error)

	// Delete removes an artifact from the registry
	// Note: This method is deprecated - use service.Delete instead
	Delete(name, version string) error

	// Validate checks if the artifact is valid for this registry type
	Validate(artifact *types.Artifact, content []byte) error

	// GetMetadata extracts metadata from the artifact content
	GetMetadata(content []byte) (map[string]interface{}, error)

	// GenerateStoragePath creates the storage path for an artifact
	GenerateStoragePath(name, version string) string
}
