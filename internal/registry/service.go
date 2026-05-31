package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry/upstream"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/config"
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
	Settings  *RegistrySettingsService
	factory   *Factory
	handlers  map[string]Handler
	upstream  *upstream.Service
}

// NewService creates a new registry service
func NewService(db *common.Database, storage storage.BlobStorage, proxyCfg *config.ProxyConfig) *Service {
	effectiveProxyConfig := config.ProxyConfig{}
	if proxyCfg != nil {
		effectiveProxyConfig = *proxyCfg
	}

	service := &Service{
		DB:        db,
		Storage:   storage,
		Ownership: NewOwnershipService(db.DB),
		Settings:  NewRegistrySettingsService(db.DB),
		handlers:  make(map[string]Handler),
		upstream:  upstream.NewService(effectiveProxyConfig),
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

	// Check if registry is enabled
	enabled, err := s.Settings.IsRegistryEnabled(ctx, registryType)
	if err != nil {
		log.Error().Err(err).Str("registry_type", registryType).Msg("Failed to check registry status")
		return nil, fmt.Errorf("failed to check registry status: %w", err)
	}
	if !enabled {
		log.Warn().Str("registry_type", registryType).Msg("Upload rejected - registry is disabled")
		return nil, fmt.Errorf("registry %s is currently disabled", registryType)
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
	}

	// Check package ownership permissions
	canPublish, err := s.Ownership.CanUserPublish(ctx, registryType, artifact.Name, publishedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to check ownership permissions: %w", err)
	}

	if !canPublish {
		return nil, fmt.Errorf("insufficient permissions to publish to package %s", artifact.Name)
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
		_ = s.Storage.Delete(ctx, artifact.StoragePath)
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

	// Check if registry is enabled
	enabled, err := s.Settings.IsRegistryEnabled(ctx, registryType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check registry status: %w", err)
	}
	if !enabled {
		return nil, nil, fmt.Errorf("registry %s is currently disabled", registryType)
	}

	sanitizedName := utils.SanitizePackageName(name, registryType)

	loadArtifact := func() (*types.Artifact, error) {
		var loaded types.Artifact
		if err := s.DB.Where("LOWER(name) = LOWER(?) AND version = ? AND registry = ?",
			sanitizedName, version, registryType).First(&loaded).Error; err != nil {
			return nil, err
		}
		return &loaded, nil
	}

	artifact, err := loadArtifact()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if upstreamErr := s.ensureUpstreamCached(ctx, registryType, name, version, "artifact"); upstreamErr == nil {
				artifact, err = loadArtifact()
				if err == nil {
					// cached successfully and found locally
				} else if err != gorm.ErrRecordNotFound {
					return nil, nil, fmt.Errorf("failed to get artifact after caching: %w", err)
				}
			}
			if err == gorm.ErrRecordNotFound {
				return nil, nil, fmt.Errorf("artifact not found: %s:%s", name, version)
			}
		} else {
			return nil, nil, fmt.Errorf("failed to get artifact: %w", err)
		}
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
	s.DB.Model(artifact).Where("id = ?", artifact.ID).Update("downloads", gorm.Expr("downloads + ?", 1))

	return artifact, content, nil
}

// EnsureUpstreamCached pulls and stores an upstream artifact into local storage when configured.
func (s *Service) EnsureUpstreamCached(ctx context.Context, registryType, name, version, resource string) error {
	return s.ensureUpstreamCached(ctx, registryType, name, version, resource)
}

func (s *Service) ensureUpstreamCached(ctx context.Context, registryType, name, version, resource string) error {
	if s.upstream == nil {
		return upstream.ErrProxyDisabled
	}

	fetched, err := s.upstream.Fetch(ctx, upstream.ProxyRequest{
		Registry: registryType,
		Resource: resource,
		Name:     name,
		Version:  version,
	})
	if err != nil {
		if !errors.Is(err, upstream.ErrProxyDisabled) && !errors.Is(err, upstream.ErrUpstreamNotFound) {
			log.Warn().
				Err(err).
				Str("registry", registryType).
				Str("name", name).
				Str("version", version).
				Str("resource", resource).
				Msg("upstream fetch failed")
		}
		return err
	}

	_, err = s.cacheUpstreamArtifact(ctx, registryType, name, version, fetched)
	return err
}

func (s *Service) cacheUpstreamArtifact(ctx context.Context, registryType, name, version string, fetched *upstream.FetchedArtifact) (*types.Artifact, error) {
	handler, exists := s.handlers[registryType]
	if !exists {
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}

	sanitizedName := utils.SanitizePackageName(name, registryType)
	var existing types.Artifact
	if err := s.DB.Where("LOWER(name) = LOWER(?) AND version = ? AND registry = ?",
		sanitizedName, version, registryType).First(&existing).Error; err == nil {
		return &existing, nil
	}

	artifact := &types.Artifact{
		ID:          uuid.New(),
		Name:        sanitizedName,
		Version:     version,
		Registry:    registryType,
		ContentType: fetched.ContentType,
		Size:        int64(len(fetched.Content)),
		SHA256:      utils.ComputeSHA256(fetched.Content),
		PublishedBy: uuid.Nil,
		IsPublic:    true,
	}

	validationArtifact := *artifact
	if registryType == "rubygems" && !strings.HasSuffix(validationArtifact.Name, ".gem") {
		validationArtifact.Name = validationArtifact.Name + ".gem"
	}

	if err := handler.Validate(&validationArtifact, fetched.Content); err != nil {
		return nil, fmt.Errorf("upstream validation failed: %w", err)
	}

	metadata, err := handler.GetMetadata(fetched.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract upstream metadata: %w", err)
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["proxy_cached"] = true
	metadata["proxy_cached_at"] = time.Now().UTC().Format(time.RFC3339)
	metadata["proxy_upstream_url"] = fetched.SourceURL
	artifact.Metadata = metadata

	artifact.StoragePath = handler.GenerateStoragePath(name, version)
	if err := handler.Upload(ctx, artifact, fetched.Content); err != nil {
		return nil, fmt.Errorf("failed to store upstream artifact: %w", err)
	}

	if err := s.DB.Create(artifact).Error; err != nil {
		_ = s.Storage.Delete(ctx, artifact.StoragePath)
		return nil, fmt.Errorf("failed to persist cached artifact metadata: %w", err)
	}

	log.Info().
		Str("registry", registryType).
		Str("name", artifact.Name).
		Str("version", artifact.Version).
		Int("size", len(fetched.Content)).
		Msg("cached artifact from upstream")

	// For OCI manifests, also write digest-addressable record when possible
	if registryType == "oci" && artifact.SHA256 != "" && !strings.HasPrefix(version, "sha256:") {
		digestArtifact := *artifact
		digestArtifact.ID = uuid.New()
		digestArtifact.Version = "sha256:" + artifact.SHA256
		digestArtifact.StoragePath = handler.GenerateStoragePath(name, digestArtifact.Version)
		if err := handler.Upload(ctx, &digestArtifact, fetched.Content); err == nil {
			_ = s.DB.Create(&digestArtifact).Error
		}
	}

	return artifact, nil
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

// Delete removes an artifact. An empty version deletes every version of the
// package (used by unpublish-style flows).
func (s *Service) Delete(ctx context.Context, registryType, name, version string, userID uuid.UUID) error {
	query := s.DB.Where("LOWER(name) = LOWER(?) AND registry = ?", name, registryType)
	if version != "" {
		query = query.Where("version = ?", version)
	}

	var artifacts []types.Artifact
	if err := query.Find(&artifacts).Error; err != nil {
		return fmt.Errorf("failed to get artifact: %w", err)
	}

	if len(artifacts) == 0 {
		return fmt.Errorf("artifact not found: %s:%s", name, version)
	}

	// Check ownership permissions once against the resolved package name.
	canDelete, err := s.Ownership.CanUserDelete(ctx, registryType, artifacts[0].Name, userID)
	if err != nil {
		return fmt.Errorf("failed to check delete permissions: %w", err)
	}

	if !canDelete {
		return fmt.Errorf("insufficient permissions to delete artifact")
	}

	for i := range artifacts {
		artifact := &artifacts[i]
		if err := s.Storage.Delete(ctx, artifact.StoragePath); err != nil {
			return fmt.Errorf("failed to delete artifact from storage: %w", err)
		}
		if err := s.DB.Delete(artifact).Error; err != nil {
			return fmt.Errorf("failed to delete artifact from database: %w", err)
		}
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
