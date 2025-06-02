package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// PackageOwnershipRole constants
const (
	RoleOwner       = "owner"       // Full control: publish, delete, manage owners
	RoleMaintainer  = "maintainer"  // Publish and update packages
	RoleContributor = "contributor" // Read-only access (for future use)
)

// OwnershipService handles package ownership operations
type OwnershipService struct {
	db *gorm.DB
}

// NewOwnershipService creates a new ownership service
func NewOwnershipService(db *gorm.DB) *OwnershipService {
	return &OwnershipService{db: db}
}

// generatePackageKey creates a unique key for a package across registries
func generatePackageKey(registry, packageName string) string {
	return fmt.Sprintf("%s:%s", registry, packageName)
}

// CanUserPublish checks if a user can publish a new version of a package
func (os *OwnershipService) CanUserPublish(ctx context.Context, registry, packageName string, userID uuid.UUID) (bool, error) {
	packageKey := generatePackageKey(registry, packageName)

	// Check if user is admin
	var user types.User
	if err := os.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	if user.IsAdmin {
		return true, nil
	}

	// Check if package has any owners
	var ownershipCount int64
	if err := os.db.WithContext(ctx).Model(&types.PackageOwnership{}).
		Where("package_key = ?", packageKey).
		Count(&ownershipCount).Error; err != nil {
		return false, fmt.Errorf("failed to count package owners: %w", err)
	}

	// If no owners exist, check if user published the first version
	if ownershipCount == 0 {
		// For new packages, anyone can publish initially and becomes the owner
		return true, nil
	}

	// Check if user has publish permissions (owner or maintainer)
	var ownership types.PackageOwnership
	if err := os.db.WithContext(ctx).Where("package_key = ? AND user_id = ? AND role IN (?)",
		packageKey, userID, []string{RoleOwner, RoleMaintainer}).First(&ownership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil // User has no permissions
		}
		return false, fmt.Errorf("failed to check ownership: %w", err)
	}

	return true, nil
}

// CanUserDelete checks if a user can delete a package version
func (os *OwnershipService) CanUserDelete(ctx context.Context, registry, packageName string, userID uuid.UUID) (bool, error) {
	packageKey := generatePackageKey(registry, packageName)

	// Check if user is admin
	var user types.User
	if err := os.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	if user.IsAdmin {
		return true, nil
	}

	// Only owners can delete packages
	var ownership types.PackageOwnership
	if err := os.db.WithContext(ctx).Where("package_key = ? AND user_id = ? AND role = ?",
		packageKey, userID, RoleOwner).First(&ownership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil // User is not an owner
		}
		return false, fmt.Errorf("failed to check ownership: %w", err)
	}

	return true, nil
}

// CanUserManageOwnership checks if a user can add/remove other owners
func (os *OwnershipService) CanUserManageOwnership(ctx context.Context, registry, packageName string, userID uuid.UUID) (bool, error) {
	packageKey := generatePackageKey(registry, packageName)

	// Check if user is admin
	var user types.User
	if err := os.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	if user.IsAdmin {
		return true, nil
	}

	// Only owners can manage ownership
	var ownership types.PackageOwnership
	if err := os.db.WithContext(ctx).Where("package_key = ? AND user_id = ? AND role = ?",
		packageKey, userID, RoleOwner).First(&ownership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check ownership: %w", err)
	}

	return true, nil
}

// AddOwner adds a new owner/maintainer to a package
func (os *OwnershipService) AddOwner(ctx context.Context, registry, packageName string, targetUserID, grantedByUserID uuid.UUID, role string) error {
	// Validate role
	if role != RoleOwner && role != RoleMaintainer && role != RoleContributor {
		return fmt.Errorf("invalid role: %s", role)
	}

	// Check if granting user has permission
	canManage, err := os.CanUserManageOwnership(ctx, registry, packageName, grantedByUserID)
	if err != nil {
		return fmt.Errorf("failed to check permissions: %w", err)
	}

	if !canManage {
		return fmt.Errorf("insufficient permissions to manage package ownership")
	}

	packageKey := generatePackageKey(registry, packageName)

	// Check if ownership already exists
	var existingOwnership types.PackageOwnership
	if err := os.db.WithContext(ctx).Where("package_key = ? AND user_id = ?",
		packageKey, targetUserID).First(&existingOwnership).Error; err == nil {
		// Update existing role
		existingOwnership.Role = role
		existingOwnership.GrantedBy = grantedByUserID
		existingOwnership.GrantedAt = time.Now()
		return os.db.WithContext(ctx).Save(&existingOwnership).Error
	}

	// Create new ownership
	ownership := &types.PackageOwnership{
		ID:         uuid.New(),
		PackageKey: packageKey,
		UserID:     targetUserID,
		Role:       role,
		GrantedBy:  grantedByUserID,
		GrantedAt:  time.Now(),
	}

	if err := os.db.WithContext(ctx).Create(ownership).Error; err != nil {
		return fmt.Errorf("failed to create ownership: %w", err)
	}

	log.Info().
		Str("package_key", packageKey).
		Str("target_user_id", targetUserID.String()).
		Str("granted_by_user_id", grantedByUserID.String()).
		Str("role", role).
		Msg("Package ownership granted")

	return nil
}

// RemoveOwner removes ownership from a package
func (os *OwnershipService) RemoveOwner(ctx context.Context, registry, packageName string, targetUserID, removedByUserID uuid.UUID) error {
	packageKey := generatePackageKey(registry, packageName)

	// Check if removing user has permission
	canManage, err := os.CanUserManageOwnership(ctx, registry, packageName, removedByUserID)
	if err != nil {
		return fmt.Errorf("failed to check permissions: %w", err)
	}

	if !canManage {
		return fmt.Errorf("insufficient permissions to manage package ownership")
	}

	// Prevent removing the last owner
	var ownerCount int64
	if err := os.db.WithContext(ctx).Model(&types.PackageOwnership{}).
		Where("package_key = ? AND role = ?", packageKey, RoleOwner).
		Count(&ownerCount).Error; err != nil {
		return fmt.Errorf("failed to count owners: %w", err)
	}

	if ownerCount <= 1 {
		return fmt.Errorf("cannot remove the last owner of a package")
	}

	// Remove ownership
	if err := os.db.WithContext(ctx).Where("package_key = ? AND user_id = ?",
		packageKey, targetUserID).Delete(&types.PackageOwnership{}).Error; err != nil {
		return fmt.Errorf("failed to remove ownership: %w", err)
	}

	log.Info().
		Str("package_key", packageKey).
		Str("target_user_id", targetUserID.String()).
		Str("removed_by_user_id", removedByUserID.String()).
		Msg("Package ownership removed")

	return nil
}

// GetPackageOwners returns all owners of a package
func (os *OwnershipService) GetPackageOwners(ctx context.Context, registry, packageName string) ([]types.PackageOwnership, error) {
	packageKey := generatePackageKey(registry, packageName)

	var ownerships []types.PackageOwnership
	if err := os.db.WithContext(ctx).Where("package_key = ?", packageKey).Find(&ownerships).Error; err != nil {
		return nil, fmt.Errorf("failed to get package owners: %w", err)
	}

	return ownerships, nil
}

// EstablishInitialOwnership creates initial ownership when a package is first published
func (os *OwnershipService) EstablishInitialOwnership(ctx context.Context, registry, packageName string, userID uuid.UUID) error {
	packageKey := generatePackageKey(registry, packageName)

	// Check if ownership already exists
	var ownershipCount int64
	if err := os.db.WithContext(ctx).Model(&types.PackageOwnership{}).
		Where("package_key = ?", packageKey).
		Count(&ownershipCount).Error; err != nil {
		return fmt.Errorf("failed to count existing ownership: %w", err)
	}

	if ownershipCount > 0 {
		// Ownership already established
		return nil
	}

	// Create initial ownership
	ownership := &types.PackageOwnership{
		ID:         uuid.New(),
		PackageKey: packageKey,
		UserID:     userID,
		Role:       RoleOwner,
		GrantedBy:  userID, // Self-granted for initial ownership
		GrantedAt:  time.Now(),
	}

	if err := os.db.WithContext(ctx).Create(ownership).Error; err != nil {
		return fmt.Errorf("failed to establish initial ownership: %w", err)
	}

	log.Info().
		Str("package_key", packageKey).
		Str("user_id", userID.String()).
		Msg("Initial package ownership established")

	return nil
}

// GetUserPackages returns all packages a user has access to
func (os *OwnershipService) GetUserPackages(ctx context.Context, userID uuid.UUID) ([]types.PackageOwnership, error) {
	var ownerships []types.PackageOwnership
	if err := os.db.WithContext(ctx).Where("user_id = ?", userID).Find(&ownerships).Error; err != nil {
		return nil, fmt.Errorf("failed to get user packages: %w", err)
	}

	return ownerships, nil
}
