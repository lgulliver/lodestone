package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
	"gorm.io/gorm"
)

// Service handles authentication operations
type Service struct {
	db     *common.Database
	cache  *common.Cache
	config *config.AuthConfig
}

// NewService creates a new authentication service
func NewService(db *common.Database, cache *common.Cache, config *config.AuthConfig) *Service {
	return &Service{
		db:     db,
		cache:  cache,
		config: config,
	}
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req *types.RegisterRequest) (*types.User, error) {
	// Check if user already exists
	var existingUser types.User
	if err := s.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		return nil, fmt.Errorf("user with username or email already exists")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password, s.config.BCryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &types.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		IsActive: true,
		IsAdmin:  false,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Remove password from response
	user.Password = ""
	return user, nil
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(ctx context.Context, req *types.LoginRequest) (*types.AuthToken, error) {
	// Find user
	var user types.User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify password
	if !utils.CheckPassword(req.Password, user.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate JWT token
	token, err := utils.GenerateJWT(user.ID, s.config.JWTSecret, s.config.JWTExpiration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	authToken := &types.AuthToken{
		Token:     token,
		ExpiresAt: time.Now().Add(s.config.JWTExpiration),
		UserID:    user.ID,
	}

	// Cache the token
	cacheKey := fmt.Sprintf("token:%s", user.ID.String())
	if err := s.cache.Set(ctx, cacheKey, authToken, s.config.JWTExpiration); err != nil {
		// Log error but don't fail the login
		fmt.Printf("Failed to cache token: %v\n", err)
	}

	return authToken, nil
}

// ValidateToken validates a JWT token and returns the user
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*types.User, error) {
	// Validate JWT
	userID, err := utils.ValidateJWT(tokenString, s.config.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Try cache first
	cacheKey := fmt.Sprintf("user:%s", userID.String())
	var user types.User
	if err := s.cache.Get(ctx, cacheKey, &user); err == nil {
		return &user, nil
	}

	// Get user from database
	if err := s.db.Where("id = ? AND is_active = ?", userID, true).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache user for future requests
	if err := s.cache.Set(ctx, cacheKey, &user, 10*time.Minute); err != nil {
		fmt.Printf("Failed to cache user: %v\n", err)
	}

	user.Password = "" // Remove password from response
	return &user, nil
}

// CreateAPIKey creates a new API key for a user
func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string, permissions []string) (*types.APIKey, string, error) {
	// Generate API key
	keyValue, err := utils.GenerateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the key for storage
	keyHash := utils.HashAPIKey(keyValue)

	// Create API key record
	apiKey := &types.APIKey{
		UserID:      userID,
		Name:        name,
		KeyHash:     keyHash,
		Permissions: permissions,
		IsActive:    true,
	}

	if err := s.db.Create(apiKey).Error; err != nil {
		return nil, "", fmt.Errorf("failed to create API key: %w", err)
	}

	// Load user relationship
	if err := s.db.Preload("User").First(apiKey, apiKey.ID).Error; err != nil {
		return nil, "", fmt.Errorf("failed to load API key: %w", err)
	}

	return apiKey, keyValue, nil
}

// ValidateAPIKey validates an API key and returns the associated user
func (s *Service) ValidateAPIKey(ctx context.Context, keyValue string) (*types.User, *types.APIKey, error) {
	keyHash := utils.HashAPIKey(keyValue)

	var apiKey types.APIKey
	if err := s.db.Preload("User").Where("key_hash = ? AND is_active = ?", keyHash, true).First(&apiKey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("invalid API key")
		}
		return nil, nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	// Check if API key has expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, nil, fmt.Errorf("API key has expired")
	}

	// Check if user is active
	if !apiKey.User.IsActive {
		return nil, nil, fmt.Errorf("user account is disabled")
	}

	// Update last used timestamp
	now := time.Now()
	apiKey.LastUsedAt = &now
	s.db.Save(&apiKey)

	apiKey.User.Password = "" // Remove password from response
	return &apiKey.User, &apiKey, nil
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID uuid.UUID) (*types.User, error) {
	var user types.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.Password = "" // Remove password from response
	return &user, nil
}

// ListAPIKeys lists API keys for a user
func (s *Service) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]*types.APIKey, error) {
	var apiKeys []*types.APIKey
	if err := s.db.Where("user_id = ?", userID).Find(&apiKeys).Error; err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	return apiKeys, nil
}

// RevokeAPIKey deactivates an API key
func (s *Service) RevokeAPIKey(ctx context.Context, keyID uuid.UUID, userID uuid.UUID) error {
	result := s.db.Model(&types.APIKey{}).
		Where("id = ? AND user_id = ?", keyID, userID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to revoke API key: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}
