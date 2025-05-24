package routes

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/types"
)

// RegistryServiceInterface defines the contract for registry services
type RegistryServiceInterface interface {
	Upload(ctx context.Context, registryType, name, version string, content io.Reader, publishedBy uuid.UUID) (*types.Artifact, error)
	List(ctx context.Context, filter *types.ArtifactFilter) ([]*types.Artifact, int64, error)
	Delete(ctx context.Context, registryType, name, version string, userID uuid.UUID) error
	Download(ctx context.Context, registryType, name, version string) (*types.Artifact, io.ReadCloser, error)
}
