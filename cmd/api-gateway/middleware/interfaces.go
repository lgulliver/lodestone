package middleware

import (
	"context"

	"github.com/lgulliver/lodestone/pkg/types"
)

// AuthServiceInterface defines the contract for authentication services
type AuthServiceInterface interface {
	ValidateToken(ctx context.Context, token string) (*types.User, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (*types.User, *types.APIKey, error)
}
