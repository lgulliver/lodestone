package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testUIAuthConfig() *config.AuthConfig {
	return &config.AuthConfig{
		JWTSecret:           "test-secret",
		JWTExpiration:       time.Hour,
		BCryptCost:          4,
		UISessionCookieName: "lodestone_ui_session",
		UICSRFCookieName:    "lodestone_ui_csrf",
		UISessionExpiration: time.Hour,
		UICookieSameSite:    "Lax",
	}
}

func TestUIAuthMiddleware_ValidSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	mockAuth.On("ValidateToken", mock.Anything, "session-token").Return(user, nil)

	router := gin.New()
	router.Use(UIAuthMiddleware(mockAuth, testUIAuthConfig()))
	router.GET("/ui/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/session", nil)
	req.AddCookie(&http.Cookie{Name: "lodestone_ui_session", Value: "session-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestUIAuthMiddleware_RejectsMissingCSRFFromUnsafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	mockAuth.On("ValidateToken", mock.Anything, "session-token").Return(user, nil)

	router := gin.New()
	router.Use(UIAuthMiddleware(mockAuth, testUIAuthConfig()))
	router.POST("/ui/api-keys", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"status": "created"})
	})

	req := httptest.NewRequest(http.MethodPost, "/ui/api-keys", nil)
	req.AddCookie(&http.Cookie{Name: "lodestone_ui_session", Value: "session-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "csrf validation failed")
	mockAuth.AssertExpectations(t)
}

func TestUIAuthMiddleware_AcceptsValidCSRFToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	mockAuth.On("ValidateToken", mock.Anything, "session-token").Return(user, nil)

	router := gin.New()
	router.Use(UIAuthMiddleware(mockAuth, testUIAuthConfig()))
	router.DELETE("/ui/api-keys/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	req := httptest.NewRequest(http.MethodDelete, "/ui/api-keys/test", nil)
	req.AddCookie(&http.Cookie{Name: "lodestone_ui_session", Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: "lodestone_ui_csrf", Value: "csrf-token"})
	req.Header.Set("X-CSRF-Token", "csrf-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestUIAuthMiddleware_InvalidSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuth := new(MockAuthService)
	mockAuth.On("ValidateToken", mock.Anything, "bad-session").Return(nil, errors.New("invalid token"))

	router := gin.New()
	router.Use(UIAuthMiddleware(mockAuth, testUIAuthConfig()))
	router.GET("/ui/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/session", nil)
	req.AddCookie(&http.Cookie{Name: "lodestone_ui_session", Value: "bad-session"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

var _ AuthServiceInterface = (*MockAuthService)(nil)
