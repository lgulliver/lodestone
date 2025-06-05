package registry

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// RegistrySettingsService handles runtime configuration of registry formats
type RegistrySettingsService struct {
	db *gorm.DB
}

// NewRegistrySettingsService creates a new registry settings service
func NewRegistrySettingsService(db *gorm.DB) *RegistrySettingsService {
	return &RegistrySettingsService{db: db}
}

// IsRegistryEnabled checks if a registry format is enabled
func (s *RegistrySettingsService) IsRegistryEnabled(ctx context.Context, registryName string) (bool, error) {
	var setting types.RegistrySetting
	err := s.db.WithContext(ctx).
		Where("registry_name = ?", registryName).
		First(&setting).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Warn().
				Str("registry", registryName).
				Msg("registry setting not found, defaulting to disabled")
			return false, nil
		}
		return false, fmt.Errorf("failed to check registry status: %w", err)
	}

	return setting.Enabled, nil
}

// EnableRegistry enables a registry format
func (s *RegistrySettingsService) EnableRegistry(ctx context.Context, registryName string, updatedBy uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Model(&types.RegistrySetting{}).
		Where("registry_name = ?", registryName).
		Updates(map[string]interface{}{
			"enabled":    true,
			"updated_by": updatedBy,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to enable registry %s: %w", registryName, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("registry %s not found", registryName)
	}

	log.Info().
		Str("registry", registryName).
		Str("updated_by", updatedBy.String()).
		Msg("registry enabled")

	return nil
}

// DisableRegistry disables a registry format
func (s *RegistrySettingsService) DisableRegistry(ctx context.Context, registryName string, updatedBy uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Model(&types.RegistrySetting{}).
		Where("registry_name = ?", registryName).
		Updates(map[string]interface{}{
			"enabled":    false,
			"updated_by": updatedBy,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to disable registry %s: %w", registryName, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("registry %s not found", registryName)
	}

	log.Info().
		Str("registry", registryName).
		Str("updated_by", updatedBy.String()).
		Msg("registry disabled")

	return nil
}

// GetRegistrySettings returns all registry settings
func (s *RegistrySettingsService) GetRegistrySettings(ctx context.Context) ([]types.RegistrySetting, error) {
	var settings []types.RegistrySetting
	err := s.db.WithContext(ctx).
		Preload("UpdatedByUser").
		Order("registry_name").
		Find(&settings).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get registry settings: %w", err)
	}

	return settings, nil
}

// GetRegistrySetting returns a specific registry setting
func (s *RegistrySettingsService) GetRegistrySetting(ctx context.Context, registryName string) (*types.RegistrySetting, error) {
	var setting types.RegistrySetting
	err := s.db.WithContext(ctx).
		Preload("UpdatedByUser").
		Where("registry_name = ?", registryName).
		First(&setting).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("registry %s not found", registryName)
		}
		return nil, fmt.Errorf("failed to get registry setting: %w", err)
	}

	return &setting, nil
}

// UpdateRegistryDescription updates the description of a registry
func (s *RegistrySettingsService) UpdateRegistryDescription(ctx context.Context, registryName, description string, updatedBy uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Model(&types.RegistrySetting{}).
		Where("registry_name = ?", registryName).
		Updates(map[string]interface{}{
			"description": description,
			"updated_by":  updatedBy,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update registry description: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("registry %s not found", registryName)
	}

	log.Info().
		Str("registry", registryName).
		Str("description", description).
		Str("updated_by", updatedBy.String()).
		Msg("registry description updated")

	return nil
}
