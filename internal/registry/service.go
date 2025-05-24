package registry

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
	"gorm.io/gorm"
)

// Service handles registry operations
type Service struct {
	db        *common.Database
	storage   storage.BlobStorage
	registries map[string]types.Registry
}

// NewService creates a new registry service
func NewService(db *common.Database, storage storage.BlobStorage) *Service {
	service := &Service{
		db:         db,
		storage:    storage,
		registries: make(map[string]types.Registry),
	}

	// Register built-in registry handlers
	service.registerHandlers()
	return service
}

// registerHandlers registers all supported registry types
func (s *Service) registerHandlers() {
	s.registries["nuget"] = &NuGetRegistry{service: s}
	s.registries["npm"] = &NPMRegistry{service: s}
	s.registries["maven"] = &MavenRegistry{service: s}
	s.registries["go"] = &GoRegistry{service: s}
	s.registries["helm"] = &HelmRegistry{service: s}
	s.registries["oci"] = &OCIRegistry{service: s}
	s.registries["opa"] = &OPARegistry{service: s}
	s.registries["cargo"] = &CargoRegistry{service: s}
	s.registries["rubygems"] = &RubyGemsRegistry{service: s}
}

// Upload handles artifact upload
func (s *Service) Upload(ctx context.Context, registryType, name, version string, content io.Reader, publishedBy uuid.UUID) (*types.Artifact, error) {
	// Get registry handler
	registry, exists := s.registries[registryType]
	if !exists {
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}

	// Read content into memory for processing
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	// Create artifact object
	artifact := &types.Artifact{
		Name:        utils.SanitizePackageName(name),
		Version:     version,
		Registry:    registryType,
		Size:        int64(len(contentBytes)),
		SHA256:      utils.ComputeSHA256(contentBytes),
		PublishedBy: publishedBy,
		IsPublic:    false, // Default to private
	}

	// Validate with registry-specific handler
	if err := registry.Validate(artifact, contentBytes); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Extract metadata
	metadata, err := registry.GetMetadata(contentBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}
	artifact.Metadata = metadata

	// Check if artifact already exists
	var existingArtifact types.Artifact
	if err := s.db.Where("name = ? AND version = ? AND registry = ?", 
		artifact.Name, artifact.Version, artifact.Registry).First(&existingArtifact).Error; err == nil {
		return nil, fmt.Errorf("artifact %s:%s already exists", name, version)
	}

	// Generate storage path
	artifact.StoragePath = s.generateStoragePath(registryType, name, version)

	// Store the artifact
	if err := registry.Upload(artifact, contentBytes); err != nil {
		return nil, fmt.Errorf("failed to upload artifact: %w", err)
	}

	// Save to database
	if err := s.db.Create(artifact).Error; err != nil {
		// Try to clean up stored file on database error
		s.storage.Delete(ctx, artifact.StoragePath)
		return nil, fmt.Errorf("failed to save artifact metadata: %w", err)
	}

	return artifact, nil
}

// Download handles artifact download
func (s *Service) Download(ctx context.Context, registryType, name, version string) (*types.Artifact, io.ReadCloser, error) {
	// Get registry handler
	registry, exists := s.registries[registryType]
	if !exists {
		return nil, nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}

	// Get artifact metadata from database
	var artifact types.Artifact
	if err := s.db.Where("name = ? AND version = ? AND registry = ?", 
		name, version, registryType).First(&artifact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("artifact not found: %s:%s", name, version)
		}
		return nil, nil, fmt.Errorf("failed to get artifact: %w", err)
	}

	// Get content from storage
	content, err := s.storage.Retrieve(ctx, artifact.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve artifact: %w", err)
	}

	// Increment download counter
	s.db.Model(&artifact).Update("downloads", gorm.Expr("downloads + ?", 1))

	return &artifact, content, nil
}

// List returns artifacts matching the filter
func (s *Service) List(ctx context.Context, filter *types.ArtifactFilter) ([]*types.Artifact, int64, error) {
	query := s.db.Model(&types.Artifact{})

	// Apply filters
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.Registry != "" {
		query = query.Where("registry = ?", filter.Registry)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count artifacts: %w", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Get artifacts
	var artifacts []*types.Artifact
	if err := query.Preload("Publisher").Find(&artifacts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list artifacts: %w", err)
	}

	return artifacts, total, nil
}

// Delete removes an artifact
func (s *Service) Delete(ctx context.Context, registryType, name, version string, userID uuid.UUID) error {
	// Get artifact
	var artifact types.Artifact
	if err := s.db.Where("name = ? AND version = ? AND registry = ?", 
		name, version, registryType).First(&artifact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("artifact not found: %s:%s", name, version)
		}
		return fmt.Errorf("failed to get artifact: %w", err)
	}

	// Check permissions (user must be the publisher or an admin)
	var user types.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if artifact.PublishedBy != userID && !user.IsAdmin {
		return fmt.Errorf("insufficient permissions to delete artifact")
	}

	// Delete from storage
	if err := s.storage.Delete(ctx, artifact.StoragePath); err != nil {
		return fmt.Errorf("failed to delete artifact from storage: %w", err)
	}

	// Delete from database
	if err := s.db.Delete(&artifact).Error; err != nil {
		return fmt.Errorf("failed to delete artifact from database: %w", err)
	}

	return nil
}

// generateStoragePath creates a storage path for an artifact
func (s *Service) generateStoragePath(registryType, name, version string) string {
	// Create a hierarchical path: registry/name/version/filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s-%s.artifact", name, version, timestamp)
	return filepath.Join(registryType, name, version, filename)
}

// GetRegistry returns a registry handler by type
func (s *Service) GetRegistry(registryType string) (types.Registry, error) {
	registry, exists := s.registries[registryType]
	if !exists {
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}
	return registry, nil
}
