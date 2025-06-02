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
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Service handles registry operations
type Service struct {
	DB        *common.Database
	Storage   storage.BlobStorage
	Ownership *OwnershipService
	factory   *Factory
	handlers  map[string]Handler
}

// NewService creates a new registry service
func NewService(db *common.Database, storage storage.BlobStorage) *Service {
	service := &Service{
		DB:        db,
		Storage:   storage,
		Ownership: NewOwnershipService(db.DB),
		handlers:  make(map[string]Handler),
	}

	// Create registry factory
	service.factory = NewFactory(service)

	// Register built-in registry handlers
	service.registerHandlers()
	return service
}

// registerHandlers registers all supported registry types
func (s *Service) registerHandlers() {
	// Register handlers for all supported registries
	formats := []string{
		"nuget",
		"npm",
		"maven",
		"go",
		"helm",
		"oci",
		"opa",
		"cargo",
		"rubygems",
	}

	for _, format := range formats {
		s.handlers[format] = s.factory.GetRegistryHandler(format)
	}
}

// Upload handles artifact upload
func (s *Service) Upload(ctx context.Context, registryType, name, version string, content io.Reader, publishedBy uuid.UUID) (*types.Artifact, error) {
	log.Info().
		Str("registry_type", registryType).
		Str("name", name).
		Str("version", version).
		Str("published_by", publishedBy.String()).
		Msg("Starting artifact upload")

	// Get registry handler
	handler, exists := s.handlers[registryType]
	if !exists {
		log.Error().Str("registry_type", registryType).Msg("Unsupported registry type")
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}

	// Read content into memory for processing
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		log.Error().Err(err).Str("name", name).Msg("Failed to read artifact content")
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	log.Debug().
		Str("name", name).
		Int("content_size", len(contentBytes)).
		Msg("Artifact content read successfully")

	// Create artifact object
	artifact := &types.Artifact{
		ID:          uuid.New(), // Generate new UUID
		Name:        utils.SanitizePackageName(name, registryType),
		Version:     version,
		Registry:    registryType,
		Size:        int64(len(contentBytes)),
		SHA256:      utils.ComputeSHA256(contentBytes),
		PublishedBy: publishedBy,
		IsPublic:    false, // Default to private
	}

	// Validate with registry-specific handler
	if err := handler.Validate(artifact, contentBytes); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Extract metadata
	metadata, err := handler.GetMetadata(contentBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}
	artifact.Metadata = metadata

	// Check package ownership permissions
	canPublish, err := s.Ownership.CanUserPublish(ctx, registryType, artifact.Name, publishedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to check ownership permissions: %w", err)
	}

	if !canPublish {
		// Check if this is a new package (no existing versions)
		var existingCount int64
		if err := s.DB.Model(&types.Artifact{}).Where("LOWER(name) = LOWER(?) AND registry = ?",
			artifact.Name, artifact.Registry).Count(&existingCount).Error; err != nil {
			return nil, fmt.Errorf("failed to check existing packages: %w", err)
		}

		// If package doesn't exist, establish initial ownership
		if existingCount == 0 {
			if err := s.Ownership.EstablishInitialOwnership(ctx, registryType, artifact.Name, publishedBy); err != nil {
				return nil, fmt.Errorf("failed to establish package ownership: %w", err)
			}
		} else {
			return nil, fmt.Errorf("insufficient permissions to publish to package %s", artifact.Name)
		}
	}

	// Check if artifact already exists
	var existingArtifact types.Artifact
	if err := s.DB.Where("LOWER(name) = LOWER(?) AND version = ? AND registry = ?",
		artifact.Name, artifact.Version, artifact.Registry).First(&existingArtifact).Error; err == nil {
		return nil, fmt.Errorf("artifact %s:%s already exists", name, version)
	}

	// Generate storage path
	artifact.StoragePath = handler.GenerateStoragePath(name, version)

	// Store the artifact
	if err := handler.Upload(ctx, artifact, contentBytes); err != nil {
		return nil, fmt.Errorf("failed to upload artifact: %w", err)
	}

	// Save to database
	if err := s.DB.Create(artifact).Error; err != nil {
		// Try to clean up stored file on database error
		s.Storage.Delete(ctx, artifact.StoragePath)
		return nil, fmt.Errorf("failed to save artifact metadata: %w", err)
	}

	return artifact, nil
}

// Download handles artifact download
func (s *Service) Download(ctx context.Context, registryType, name, version string) (*types.Artifact, io.ReadCloser, error) {
	// Check if registry type is supported
	if _, exists := s.handlers[registryType]; !exists {
		return nil, nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}

	// Get artifact metadata from database
	var artifact types.Artifact
	if err := s.DB.Where("LOWER(name) = LOWER(?) AND version = ? AND registry = ?",
		name, version, registryType).First(&artifact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("artifact not found: %s:%s", name, version)
		}
		return nil, nil, fmt.Errorf("failed to get artifact: %w", err)
	}

	// Log artifact details
	log.Info().
		Str("name", artifact.Name).
		Str("version", artifact.Version).
		Str("storage_path", artifact.StoragePath).
		Int64("size", artifact.Size).
		Msg("found artifact in database")

	// Get content from storage
	content, err := s.Storage.Retrieve(ctx, artifact.StoragePath)
	if err != nil {
		log.Error().Err(err).
			Str("storage_path", artifact.StoragePath).
			Msg("failed to retrieve artifact from storage")
		return nil, nil, fmt.Errorf("failed to retrieve artifact: %w", err)
	}

	// Increment download counter
	s.DB.Model(&artifact).Where("id = ?", artifact.ID).Update("downloads", gorm.Expr("downloads + ?", 1))

	return &artifact, content, nil
}

// List returns artifacts matching the filter
func (s *Service) List(ctx context.Context, filter *types.ArtifactFilter) ([]*types.Artifact, int64, error) {
	query := s.DB.Model(&types.Artifact{})

	// Apply filters
	if filter.Name != "" {
		query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+filter.Name+"%")
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
	if err := s.DB.Where("LOWER(name) = LOWER(?) AND version = ? AND registry = ?",
		name, version, registryType).First(&artifact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("artifact not found: %s:%s", name, version)
		}
		return fmt.Errorf("failed to get artifact: %w", err)
	}

	// Check ownership permissions
	canDelete, err := s.Ownership.CanUserDelete(ctx, registryType, artifact.Name, userID)
	if err != nil {
		return fmt.Errorf("failed to check delete permissions: %w", err)
	}

	if !canDelete {
		return fmt.Errorf("insufficient permissions to delete artifact")
	}

	// Delete from storage
	if err := s.Storage.Delete(ctx, artifact.StoragePath); err != nil {
		return fmt.Errorf("failed to delete artifact from storage: %w", err)
	}

	// Delete from database
	if err := s.DB.Delete(&artifact).Error; err != nil {
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
func (s *Service) GetRegistry(registryType string) (Handler, error) {
	handler, exists := s.handlers[registryType]
	if !exists {
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}
	return handler, nil
}

// GetPackageOwners returns all owners of a package
func (s *Service) GetPackageOwners(ctx context.Context, registryType, packageName string) ([]types.PackageOwnership, error) {
	return s.Ownership.GetPackageOwners(ctx, registryType, packageName)
}

// AddPackageOwner adds a new owner to a package
func (s *Service) AddPackageOwner(ctx context.Context, registryType, packageName string, ownerUserID, targetUserID uuid.UUID, role string) error {
	// Check if the requesting user can manage ownership
	canManage, err := s.Ownership.CanUserManageOwnership(ctx, registryType, packageName, ownerUserID)
	if err != nil {
		return fmt.Errorf("failed to check management permissions: %w", err)
	}

	if !canManage {
		return fmt.Errorf("insufficient permissions to manage package ownership")
	}

	return s.Ownership.AddOwner(ctx, registryType, packageName, targetUserID, ownerUserID, role)
}

// RemovePackageOwner removes an owner from a package
func (s *Service) RemovePackageOwner(ctx context.Context, registryType, packageName string, ownerUserID, targetUserID uuid.UUID) error {
	// Check if the requesting user can manage ownership
	canManage, err := s.Ownership.CanUserManageOwnership(ctx, registryType, packageName, ownerUserID)
	if err != nil {
		return fmt.Errorf("failed to check management permissions: %w", err)
	}

	if !canManage {
		return fmt.Errorf("insufficient permissions to manage package ownership")
	}

	return s.Ownership.RemoveOwner(ctx, registryType, packageName, targetUserID, ownerUserID)
}
