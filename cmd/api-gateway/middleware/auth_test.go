package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService mocks the auth service for testing
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) ValidateToken(ctx context.Context, token string) (*types.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockAuthService) ValidateAPIKey(ctx context.Context, apiKey string) (*types.User, *types.APIKey, error) {
	args := m.Called(ctx, apiKey)
	var user *types.User
	var key *types.APIKey

	if args.Get(0) != nil {
		user = args.Get(0).(*types.User)
	}
	if args.Get(1) != nil {
		key = args.Get(1).(*types.APIKey)
	}

	return user, key, args.Error(2)
}

func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	mockAuth.On("ValidateToken", mock.Anything, "valid-token").Return(user, nil)

	var capturedNext bool
	var capturedUser *types.User

	router := gin.New()
	router.Use(authMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		capturedNext = true
		userFromContext, exists := c.Get("user")
		if exists {
			capturedUser = userFromContext.(*types.User)
		}
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedNext)
	assert.Equal(t, user, capturedUser)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_InvalidBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	mockAuth.On("ValidateToken", mock.Anything, "invalid-token").Return(nil, errors.New("invalid token"))
	// Note: ValidateAPIKey should NOT be called when there's a Bearer token

	router := gin.New()
	router.Use(authMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_ValidAPIKeyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}
	apiKey := &types.APIKey{
		ID:     uuid.New(),
		UserID: user.ID,
		Name:   "test-key",
	}

	mockAuth.On("ValidateAPIKey", mock.Anything, "valid-api-key").Return(user, apiKey, nil)

	var capturedNext bool
	var capturedUser *types.User

	router := gin.New()
	router.Use(authMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		capturedNext = true
		userFromContext, exists := c.Get("user")
		if exists {
			capturedUser = userFromContext.(*types.User)
		}
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-api-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedNext)
	assert.Equal(t, user, capturedUser)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_ValidAPIKeyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}
	apiKey := &types.APIKey{
		ID:     uuid.New(),
		UserID: user.ID,
		Name:   "test-key",
	}

	mockAuth.On("ValidateAPIKey", mock.Anything, "valid-api-key").Return(user, apiKey, nil)

	var capturedNext bool
	var capturedUser *types.User

	router := gin.New()
	router.Use(authMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		capturedNext = true
		userFromContext, exists := c.Get("user")
		if exists {
			capturedUser = userFromContext.(*types.User)
		}
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test?api_key=valid-api-key", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedNext)
	assert.Equal(t, user, capturedUser)
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)

	router := gin.New()
	router.Use(authMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOptionalAuthMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	mockAuth.On("ValidateToken", mock.Anything, "valid-token").Return(user, nil)

	var capturedNext bool
	var capturedUser *types.User

	router := gin.New()
	router.Use(optionalAuthMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		capturedNext = true
		userFromContext, exists := c.Get("user")
		if exists {
			capturedUser = userFromContext.(*types.User)
		}
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedNext)
	assert.Equal(t, user, capturedUser)
	mockAuth.AssertExpectations(t)
}

func TestOptionalAuthMiddleware_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	mockAuth.On("ValidateToken", mock.Anything, "invalid-token").Return(nil, errors.New("invalid token"))

	var capturedNext bool

	router := gin.New()
	router.Use(optionalAuthMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		capturedNext = true
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should still call next even with invalid token (optional auth)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedNext)

	// Check user was NOT set in context - we'd need to capture this in the handler
	mockAuth.AssertExpectations(t)
}

func TestOptionalAuthMiddleware_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)

	var capturedNext bool

	router := gin.New()
	router.Use(optionalAuthMiddlewareWithInterface(mockAuth))
	router.GET("/test", func(c *gin.Context) {
		capturedNext = true
		c.JSON(200, gin.H{"status": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should call next even without auth (optional auth)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedNext)
}

func TestGetUserFromContext_UserExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user", user)

	contextUser, exists := GetUserFromContext(c)

	assert.True(t, exists)
	assert.Equal(t, user, contextUser)
}

func TestGetUserFromContext_UserNotExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	contextUser, exists := GetUserFromContext(c)

	assert.False(t, exists)
	assert.Nil(t, contextUser)
}

func TestGetUserFromContext_WrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user", "not-a-user-struct")

	contextUser, exists := GetUserFromContext(c)

	assert.False(t, exists)
	assert.Nil(t, contextUser)
}
