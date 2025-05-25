package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *common.Database {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto migrate tables
	err = db.AutoMigrate(&types.User{}, &types.APIKey{})
	require.NoError(t, err)

	return &common.Database{DB: db}
}

func setupTestService(t *testing.T) (*Service, *common.Database) {
	db := setupTestDB(t)

	authConfig := &config.AuthConfig{
		JWTSecret:     "test-secret-key-for-testing-purposes",
		JWTExpiration: time.Hour,
		BCryptCost:    4, // Low cost for testing speed
	}

	service := NewService(db, nil, authConfig)
	return service, db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	authConfig := &config.AuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiration: time.Hour,
		BCryptCost:    4,
	}

	service := NewService(db, nil, authConfig)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
	assert.Nil(t, service.cache)
	assert.Equal(t, authConfig, service.config)
}

func TestRegister_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	req := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}

	user, err := service.Register(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, req.Username, user.Username)
	assert.Equal(t, req.Email, user.Email)
	assert.Empty(t, user.Password) // Password should be removed from response
	assert.True(t, user.IsActive)
	assert.False(t, user.IsAdmin)
	assert.NotEqual(t, uuid.Nil, user.ID)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	// Create initial user
	user := &types.User{
		Username: "testuser",
		Email:    "first@example.com",
		Password: "hashedpassword",
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error)

	// Try to register with same username
	req := &types.RegisterRequest{
		Username: "testuser",
		Email:    "second@example.com",
		Password: "testpassword123",
	}

	result, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user with username or email already exists")
}

func TestRegister_DuplicateEmail(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	// Create initial user
	user := &types.User{
		Username: "firstuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error)

	// Try to register with same email
	req := &types.RegisterRequest{
		Username: "seconduser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}

	result, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user with username or email already exists")
}

func TestLogin_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Login with correct credentials
	loginReq := &types.LoginRequest{
		Username: "testuser",
		Password: "testpassword123",
	}

	authToken, err := service.Login(ctx, loginReq)

	assert.NoError(t, err)
	assert.NotNil(t, authToken)
	assert.NotEmpty(t, authToken.Token)
	assert.Equal(t, user.ID, authToken.UserID)
	assert.True(t, authToken.ExpiresAt.After(time.Now()))
}

func TestLogin_InvalidUsername(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	loginReq := &types.LoginRequest{
		Username: "nonexistent",
		Password: "testpassword123",
	}

	authToken, err := service.Login(ctx, loginReq)

	assert.Error(t, err)
	assert.Nil(t, authToken)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestLogin_InvalidPassword(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	_, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Login with wrong password
	loginReq := &types.LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}

	authToken, err := service.Login(ctx, loginReq)

	assert.Error(t, err)
	assert.Nil(t, authToken)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestLogin_InactiveUser(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	// Hash the password properly
	hashedPassword, err := utils.HashPassword("testpassword123", 10)
	require.NoError(t, err)

	// Create inactive user
	user := &types.User{
		Username: "testuser",
		Email:    "test@example.com",
		Password: hashedPassword,
		IsActive: true, // Start as active
	}
	require.NoError(t, db.Create(user).Error)

	// Now explicitly set to inactive
	require.NoError(t, db.Model(user).Update("is_active", false).Error)

	// Verify user was created as inactive
	var savedUser types.User
	require.NoError(t, db.Where("username = ?", "testuser").First(&savedUser).Error)
	require.False(t, savedUser.IsActive, "User should be inactive in database")

	loginReq := &types.LoginRequest{
		Username: "testuser",
		Password: "testpassword123",
	}

	authToken, err := service.Login(ctx, loginReq)

	assert.Error(t, err)
	assert.Nil(t, authToken)
	assert.Contains(t, err.Error(), "user account is disabled")
}

func TestValidateToken_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register and login user
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	loginReq := &types.LoginRequest{
		Username: "testuser",
		Password: "testpassword123",
	}
	authToken, err := service.Login(ctx, loginReq)
	require.NoError(t, err)

	// Validate token
	validatedUser, err := service.ValidateToken(ctx, authToken.Token)

	assert.NoError(t, err)
	assert.NotNil(t, validatedUser)
	assert.Equal(t, user.ID, validatedUser.ID)
	assert.Equal(t, user.Username, validatedUser.Username)
	assert.Empty(t, validatedUser.Password) // Password should be removed
}

func TestValidateToken_InvalidToken(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	invalidToken := "invalid.jwt.token"

	user, err := service.ValidateToken(ctx, invalidToken)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestCreateAPIKey_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Create API key
	permissions := []string{"read", "write"}
	apiKey, keyValue, err := service.CreateAPIKey(ctx, user.ID, "test-key", permissions)

	assert.NoError(t, err)
	assert.NotNil(t, apiKey)
	assert.NotEmpty(t, keyValue)
	assert.Equal(t, user.ID, apiKey.UserID)
	assert.Equal(t, "test-key", apiKey.Name)
	assert.Equal(t, permissions, apiKey.Permissions)
	assert.True(t, apiKey.IsActive)
	assert.NotEmpty(t, apiKey.KeyHash)
}

func TestValidateAPIKey_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Create API key
	permissions := []string{"read", "write"}
	apiKey, keyValue, err := service.CreateAPIKey(ctx, user.ID, "test-key", permissions)
	require.NoError(t, err)

	// Validate API key
	validatedUser, validatedAPIKey, err := service.ValidateAPIKey(ctx, keyValue)

	assert.NoError(t, err)
	assert.NotNil(t, validatedUser)
	assert.NotNil(t, validatedAPIKey)
	assert.Equal(t, user.ID, validatedUser.ID)
	assert.Equal(t, user.Username, validatedUser.Username)
	assert.Equal(t, apiKey.ID, validatedAPIKey.ID)
	assert.Empty(t, validatedUser.Password) // Password should be removed
}

func TestValidateAPIKey_InvalidKey(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	invalidKey := "lodge_invalid_key"

	user, apiKey, err := service.ValidateAPIKey(ctx, invalidKey)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Nil(t, apiKey)
	assert.Contains(t, err.Error(), "invalid API key")
}

func TestValidateAPIKey_InactiveUser(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	// Create user and API key
	user := &types.User{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error)

	permissions := []string{"read"}
	_, keyValue, err := service.CreateAPIKey(ctx, user.ID, "test-key", permissions)
	require.NoError(t, err)

	// Deactivate user
	require.NoError(t, db.Model(user).Update("is_active", false).Error)

	// Try to validate API key
	validatedUser, validatedAPIKey, err := service.ValidateAPIKey(ctx, keyValue)

	assert.Error(t, err)
	assert.Nil(t, validatedUser)
	assert.Nil(t, validatedAPIKey)
	assert.Contains(t, err.Error(), "user account is disabled")
}

func TestGetUserByID_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Get user by ID
	retrievedUser, err := service.GetUserByID(ctx, user.ID)

	assert.NoError(t, err)
	assert.NotNil(t, retrievedUser)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Empty(t, retrievedUser.Password) // Password should be removed
}

func TestGetUserByID_NotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	nonExistentID := uuid.New()

	user, err := service.GetUserByID(ctx, nonExistentID)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "user not found")
}

func TestListAPIKeys_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Create multiple API keys
	_, _, err = service.CreateAPIKey(ctx, user.ID, "key1", []string{"read"})
	require.NoError(t, err)
	_, _, err = service.CreateAPIKey(ctx, user.ID, "key2", []string{"write"})
	require.NoError(t, err)

	// List API keys
	apiKeys, err := service.ListAPIKeys(ctx, user.ID)

	assert.NoError(t, err)
	assert.Len(t, apiKeys, 2)

	// Check that all API keys belong to the user
	for _, apiKey := range apiKeys {
		assert.Equal(t, user.ID, apiKey.UserID)
		assert.Empty(t, apiKey.User.Password) // Password should be removed
	}
}

func TestListAPIKeys_EmptyList(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// List API keys (should be empty)
	apiKeys, err := service.ListAPIKeys(ctx, user.ID)

	assert.NoError(t, err)
	assert.Len(t, apiKeys, 0)
}

func TestRevokeAPIKey_Success(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	// Create API key
	apiKey, _, err := service.CreateAPIKey(ctx, user.ID, "test-key", []string{"read"})
	require.NoError(t, err)

	// Revoke API key
	err = service.RevokeAPIKey(ctx, apiKey.ID, user.ID)

	assert.NoError(t, err)

	// Verify API key is inactive
	var revokedKey types.APIKey
	service.db.First(&revokedKey, apiKey.ID)
	assert.False(t, revokedKey.IsActive)
}

func TestRevokeAPIKey_NotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register a user first
	registerReq := &types.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	user, err := service.Register(ctx, registerReq)
	require.NoError(t, err)

	nonExistentKeyID := uuid.New()

	err = service.RevokeAPIKey(ctx, nonExistentKeyID, user.ID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}

func TestRevokeAPIKey_WrongUser(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Register two users
	registerReq1 := &types.RegisterRequest{
		Username: "user1",
		Email:    "user1@example.com",
		Password: "testpassword123",
	}
	user1, err := service.Register(ctx, registerReq1)
	require.NoError(t, err)

	registerReq2 := &types.RegisterRequest{
		Username: "user2",
		Email:    "user2@example.com",
		Password: "testpassword123",
	}
	user2, err := service.Register(ctx, registerReq2)
	require.NoError(t, err)

	// Create API key for user1
	apiKey, _, err := service.CreateAPIKey(ctx, user1.ID, "test-key", []string{"read"})
	require.NoError(t, err)

	// Try to revoke user1's API key as user2
	err = service.RevokeAPIKey(ctx, apiKey.ID, user2.ID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}
