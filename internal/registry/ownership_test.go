package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestOwnershipService(t *testing.T) (*OwnershipService, *common.Database) {
	// Create an in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run auto migrations
	err = db.AutoMigrate(&types.User{}, &types.Artifact{}, &types.PackageOwnership{})
	require.NoError(t, err)

	commonDB := &common.Database{DB: db}
	service := NewOwnershipService(commonDB.DB)
	return service, commonDB
}

var userCounter int = 0

func createTestUserWithAdmin(t *testing.T, db *common.Database, isAdmin bool) *types.User {
	// Use counter to ensure uniqueness
	userCounter++
	suffix := testing.TB(t).Name() + "-" + fmt.Sprintf("%d", userCounter)

	user := &types.User{
		Username: "testuser-" + suffix,
		Email:    "test-" + suffix + "@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  isAdmin,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestEstablishInitialOwnership(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	user := createTestUserWithAdmin(t, db, false)
	ctx := context.Background()

	// Test initial ownership establishment
	err := service.EstablishInitialOwnership(ctx, "npm", "test-package", user.ID)
	assert.NoError(t, err)

	// Verify ownership was created
	var ownership types.PackageOwnership
	err = db.Where("package_key = ? AND user_id = ?", "npm:test-package", user.ID).First(&ownership).Error
	assert.NoError(t, err)
	assert.Equal(t, RoleOwner, ownership.Role)
	assert.Equal(t, user.ID, ownership.GrantedBy)
	assert.NotZero(t, ownership.GrantedAt)

	// Test idempotency (establish ownership again for same package/user)
	err = service.EstablishInitialOwnership(ctx, "npm", "test-package", user.ID)
	assert.NoError(t, err)
}

func TestCanUserPublish(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	regularUser := createTestUserWithAdmin(t, db, false)
	adminUser := createTestUserWithAdmin(t, db, true)
	ctx := context.Background()

	// For new packages (no owners yet), any user should be able to publish
	canPublish, err := service.CanUserPublish(ctx, "npm", "new-package", regularUser.ID)
	assert.NoError(t, err)
	assert.True(t, canPublish)

	// Establish initial ownership
	err = service.EstablishInitialOwnership(ctx, "npm", "test-package", regularUser.ID)
	assert.NoError(t, err)

	// Owner should be able to publish
	canPublish, err = service.CanUserPublish(ctx, "npm", "test-package", regularUser.ID)
	assert.NoError(t, err)
	assert.True(t, canPublish)

	// Create another user with no permissions
	otherUser := createTestUserWithAdmin(t, db, false)
	canPublish, err = service.CanUserPublish(ctx, "npm", "test-package", otherUser.ID)
	assert.NoError(t, err)
	assert.False(t, canPublish)

	// Add other user as maintainer
	err = service.AddOwner(ctx, "npm", "test-package", otherUser.ID, regularUser.ID, RoleMaintainer)
	assert.NoError(t, err)

	// Maintainer should be able to publish
	canPublish, err = service.CanUserPublish(ctx, "npm", "test-package", otherUser.ID)
	assert.NoError(t, err)
	assert.True(t, canPublish)

	// Admin should always be able to publish
	canPublish, err = service.CanUserPublish(ctx, "npm", "test-package", adminUser.ID)
	assert.NoError(t, err)
	assert.True(t, canPublish)
}

func TestCanUserDelete(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	owner := createTestUserWithAdmin(t, db, false)
	adminUser := createTestUserWithAdmin(t, db, true)
	ctx := context.Background()

	// Establish initial ownership
	err := service.EstablishInitialOwnership(ctx, "npm", "test-package", owner.ID)
	assert.NoError(t, err)

	// Owner should be able to delete
	canDelete, err := service.CanUserDelete(ctx, "npm", "test-package", owner.ID)
	assert.NoError(t, err)
	assert.True(t, canDelete)

	// Create a maintainer
	maintainer := createTestUserWithAdmin(t, db, false)
	err = service.AddOwner(ctx, "npm", "test-package", maintainer.ID, owner.ID, RoleMaintainer)
	assert.NoError(t, err)

	// Maintainer should NOT be able to delete
	canDelete, err = service.CanUserDelete(ctx, "npm", "test-package", maintainer.ID)
	assert.NoError(t, err)
	assert.False(t, canDelete)

	// Admin should always be able to delete
	canDelete, err = service.CanUserDelete(ctx, "npm", "test-package", adminUser.ID)
	assert.NoError(t, err)
	assert.True(t, canDelete)
}

func TestCanUserManageOwnership(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	owner := createTestUserWithAdmin(t, db, false)
	adminUser := createTestUserWithAdmin(t, db, true)
	ctx := context.Background()

	// Establish initial ownership
	err := service.EstablishInitialOwnership(ctx, "npm", "test-package", owner.ID)
	assert.NoError(t, err)

	// Owner should be able to manage ownership
	canManage, err := service.CanUserManageOwnership(ctx, "npm", "test-package", owner.ID)
	assert.NoError(t, err)
	assert.True(t, canManage)

	// Create a maintainer
	maintainer := createTestUserWithAdmin(t, db, false)
	err = service.AddOwner(ctx, "npm", "test-package", maintainer.ID, owner.ID, RoleMaintainer)
	assert.NoError(t, err)

	// Maintainer should NOT be able to manage ownership
	canManage, err = service.CanUserManageOwnership(ctx, "npm", "test-package", maintainer.ID)
	assert.NoError(t, err)
	assert.False(t, canManage)

	// Admin should always be able to manage ownership
	canManage, err = service.CanUserManageOwnership(ctx, "npm", "test-package", adminUser.ID)
	assert.NoError(t, err)
	assert.True(t, canManage)
}

func TestAddOwnerAndRemoveOwner(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	owner := createTestUserWithAdmin(t, db, false)
	ctx := context.Background()

	// Establish initial ownership
	err := service.EstablishInitialOwnership(ctx, "npm", "test-package", owner.ID)
	assert.NoError(t, err)

	// Add another owner
	newOwner := createTestUserWithAdmin(t, db, false)
	err = service.AddOwner(ctx, "npm", "test-package", newOwner.ID, owner.ID, RoleOwner)
	assert.NoError(t, err)

	// Verify both are owners
	owners, err := service.GetPackageOwners(ctx, "npm", "test-package")
	assert.NoError(t, err)
	assert.Len(t, owners, 2)

	// Try to remove the first owner
	err = service.RemoveOwner(ctx, "npm", "test-package", owner.ID, newOwner.ID)
	assert.NoError(t, err)

	// Verify only one owner remains
	owners, err = service.GetPackageOwners(ctx, "npm", "test-package")
	assert.NoError(t, err)
	assert.Len(t, owners, 1)
	assert.Equal(t, newOwner.ID, owners[0].UserID)
}

func TestRemoveLastOwner(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	owner := createTestUserWithAdmin(t, db, false)
	ctx := context.Background()

	// Establish initial ownership
	err := service.EstablishInitialOwnership(ctx, "npm", "test-package", owner.ID)
	assert.NoError(t, err)

	// Try to remove the only owner - should fail
	err = service.RemoveOwner(ctx, "npm", "test-package", owner.ID, owner.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove the last owner")

	// The owner should still exist
	owners, err := service.GetPackageOwners(ctx, "npm", "test-package")
	assert.NoError(t, err)
	assert.Len(t, owners, 1)
}

func TestGetUserPackages(t *testing.T) {
	service, db := setupTestOwnershipService(t)
	user := createTestUserWithAdmin(t, db, false)
	ctx := context.Background()

	// Create two packages owned by the user
	err := service.EstablishInitialOwnership(ctx, "npm", "package-1", user.ID)
	assert.NoError(t, err)
	err = service.EstablishInitialOwnership(ctx, "nuget", "package-2", user.ID)
	assert.NoError(t, err)

	// Create another user with one package
	otherUser := createTestUserWithAdmin(t, db, false)
	err = service.EstablishInitialOwnership(ctx, "cargo", "package-3", otherUser.ID)
	assert.NoError(t, err)

	// Get packages for the first user
	packages, err := service.GetUserPackages(ctx, user.ID)
	assert.NoError(t, err)
	assert.Len(t, packages, 2)

	// Verify package keys
	packageKeys := []string{packages[0].PackageKey, packages[1].PackageKey}
	assert.Contains(t, packageKeys, "npm:package-1")
	assert.Contains(t, packageKeys, "nuget:package-2")
}

func TestGeneratePackageKey(t *testing.T) {
	testCases := []struct {
		registry string
		name     string
		expected string
	}{
		{"npm", "lodestone", "npm:lodestone"},
		{"nuget", "Lodestone.Core", "nuget:Lodestone.Core"},
		{"maven", "org.lodestone:server", "maven:org.lodestone:server"},
		{"cargo", "lodestone-client", "cargo:lodestone-client"},
	}

	for _, tc := range testCases {
		result := generatePackageKey(tc.registry, tc.name)
		assert.Equal(t, tc.expected, result)
	}
}
