package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/lgulliver/lodestone/pkg/utils"
)

func TestValidateToken_UserNotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	// Valid JWT for a user that does not exist in the DB.
	token, err := utils.GenerateJWT(uuid.New(), service.config.JWTSecret, time.Hour)
	require.NoError(t, err)

	user, err := service.ValidateToken(ctx, token)
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "user not found")
}

func TestValidateOCIToken_Malformed(t *testing.T) {
	service, _ := setupTestService(t)
	_, _, err := service.ValidateOCIToken(context.Background(), "not.a.token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid OCI token")
}

func TestAuthorizeOCITokenScope_Branches(t *testing.T) {
	service, _ := setupTestService(t)

	// nil claims
	assert.Error(t, service.AuthorizeOCITokenScope(nil, "repo", "pull"))

	// empty repository/action short-circuits to allow
	assert.NoError(t, service.AuthorizeOCITokenScope(&OCITokenClaims{}, "", "pull"))
	assert.NoError(t, service.AuthorizeOCITokenScope(&OCITokenClaims{}, "repo", ""))

	// access derived from Scope strings when Access empty
	scopeClaims := &OCITokenClaims{Scope: []string{"repository:team/app:pull,push"}}
	assert.NoError(t, service.AuthorizeOCITokenScope(scopeClaims, "team/app", "push"))

	// wildcard name + wildcard action
	wild := &OCITokenClaims{Access: []OCIAccessEntry{{Type: "repository", Name: "*", Actions: []string{"*"}}}}
	assert.NoError(t, service.AuthorizeOCITokenScope(wild, "anything", "delete"))

	// non-repository entry is skipped, then denied
	other := &OCITokenClaims{Access: []OCIAccessEntry{{Type: "registry", Name: "catalog", Actions: []string{"*"}}}}
	assert.Error(t, service.AuthorizeOCITokenScope(other, "repo", "pull"))
}

func TestParseOCIScope_Invalid(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	user, err := service.Register(ctx, &types.RegisterRequest{
		Username: "scope-user", Email: "scope@example.com", Password: "testpassword123",
	})
	require.NoError(t, err)

	// Malformed scopes are dropped; only the valid one survives in Access.
	token, _, err := service.GenerateOCIToken(user.ID, "registry",
		[]string{"bad", "repository::pull", "repository:repo:", "repository:repo:pull"}, time.Hour)
	require.NoError(t, err)

	_, claims, err := service.ValidateOCIToken(ctx, token)
	require.NoError(t, err)
	require.Len(t, claims.Access, 1)
	assert.Equal(t, "repo", claims.Access[0].Name)
}
