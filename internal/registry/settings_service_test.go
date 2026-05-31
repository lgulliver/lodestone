package registry

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

func setupSettingsService(t *testing.T) (*RegistrySettingsService, *types.User) {
	db := setupTestDB(t)
	user := &types.User{Username: "admin", Email: "a@b.c", Password: "x", IsActive: true, IsAdmin: true}
	require.NoError(t, db.Create(user).Error)
	return NewRegistrySettingsService(db.DB), user
}

func TestIsRegistryEnabled(t *testing.T) {
	s, _ := setupSettingsService(t)
	ctx := context.Background()

	enabled, err := s.IsRegistryEnabled(ctx, "npm")
	require.NoError(t, err)
	assert.True(t, enabled)

	// Unknown registry → disabled, no error.
	enabled, err = s.IsRegistryEnabled(ctx, "ghost")
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestEnableDisableRegistry(t *testing.T) {
	s, user := setupSettingsService(t)
	ctx := context.Background()

	require.NoError(t, s.DisableRegistry(ctx, "npm", user.ID))
	enabled, _ := s.IsRegistryEnabled(ctx, "npm")
	assert.False(t, enabled)

	require.NoError(t, s.EnableRegistry(ctx, "npm", user.ID))
	enabled, _ = s.IsRegistryEnabled(ctx, "npm")
	assert.True(t, enabled)

	// Not found cases.
	assert.Error(t, s.EnableRegistry(ctx, "ghost", user.ID))
	assert.Error(t, s.DisableRegistry(ctx, "ghost", user.ID))
}

func TestGetRegistrySettings(t *testing.T) {
	s, _ := setupSettingsService(t)
	settings, err := s.GetRegistrySettings(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, settings)
	found := false
	for _, st := range settings {
		if st.RegistryName == "npm" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGetRegistrySetting(t *testing.T) {
	s, _ := setupSettingsService(t)
	ctx := context.Background()

	got, err := s.GetRegistrySetting(ctx, "npm")
	require.NoError(t, err)
	assert.Equal(t, "npm", got.RegistryName)

	_, err = s.GetRegistrySetting(ctx, "ghost")
	assert.Error(t, err)
}

func TestUpdateRegistryDescription(t *testing.T) {
	s, user := setupSettingsService(t)
	ctx := context.Background()

	require.NoError(t, s.UpdateRegistryDescription(ctx, "npm", "updated desc", user.ID))
	got, _ := s.GetRegistrySetting(ctx, "npm")
	assert.Equal(t, "updated desc", got.Description)

	assert.Error(t, s.UpdateRegistryDescription(ctx, "ghost", "x", user.ID))
}

func TestStorageService(t *testing.T) {
	db := setupTestDB(t)
	storage := &MockBlobStorage{}
	svc := NewStorageService(db, storage)
	assert.Equal(t, storage, svc.GetStorage())
	assert.Equal(t, db, svc.GetDB())
}

func TestRegistrySettingBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	setting := &types.RegistrySetting{RegistryName: "uniquereg", Enabled: true}
	require.NoError(t, db.Create(setting).Error)
	assert.NotEqual(t, uuid.Nil, setting.ID)
}
