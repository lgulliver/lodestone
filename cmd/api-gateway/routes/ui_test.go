package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUIRouteTestAuthService(t *testing.T) (*auth.Service, *config.AuthConfig) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.User{}, &types.APIKey{}))

	authConfig := &config.AuthConfig{
		JWTSecret:           "ui-route-test-secret",
		JWTExpiration:       24 * time.Hour,
		BCryptCost:          4,
		UISessionCookieName: "lodestone_ui_session",
		UISessionExpiration: 2 * time.Hour,
		UICookieSecure:      false,
		UICookieSameSite:    "Lax",
	}

	return auth.NewService(&common.Database{DB: db}, nil, authConfig), authConfig
}

func registerUITestUser(t *testing.T, authService *auth.Service) {
	t.Helper()

	_, err := authService.Register(context.Background(), &types.RegisterRequest{
		Username: "ui-user",
		Email:    "ui@example.com",
		Password: "super-secret-password",
	})
	require.NoError(t, err)
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q not found", name)
	return nil
}

func newUITestRouter(authService *auth.Service, authConfig *config.AuthConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	UIRoutes(api, authService, authConfig)
	return router
}

func TestUIRoutes_LoginSetsSecureBrowserCookies(t *testing.T) {
	authService, authConfig := setupUIRouteTestAuthService(t)
	registerUITestUser(t, authService)
	router := newUITestRouter(authService, authConfig)

	body := bytes.NewBufferString(`{"username":"ui-user","password":"super-secret-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ui/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.NotContains(t, payload, "token")
	assert.Contains(t, payload, "user")

	cookies := w.Result().Cookies()
	sessionCookie := findCookie(t, cookies, authConfig.UISessionCookieName)

	assert.True(t, sessionCookie.HttpOnly)
	assert.Equal(t, uiCookiePath, sessionCookie.Path)
	assert.NotEmpty(t, w.Header().Get("X-CSRF-Token"))
}

func TestUIRoutes_SessionAndAPIKeyLifecycle(t *testing.T) {
	authService, authConfig := setupUIRouteTestAuthService(t)
	registerUITestUser(t, authService)
	router := newUITestRouter(authService, authConfig)

	loginBody := bytes.NewBufferString(`{"username":"ui-user","password":"super-secret-password"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/ui/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)
	require.Equal(t, http.StatusOK, loginResp.Code)

	cookies := loginResp.Result().Cookies()
	sessionCookie := findCookie(t, cookies, authConfig.UISessionCookieName)
	csrfToken := loginResp.Header().Get("X-CSRF-Token")
	require.NotEmpty(t, csrfToken)

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/v1/ui/auth/session", nil)
	sessionReq.AddCookie(sessionCookie)
	sessionResp := httptest.NewRecorder()
	router.ServeHTTP(sessionResp, sessionReq)

	require.Equal(t, http.StatusOK, sessionResp.Code)
	assert.Contains(t, sessionResp.Body.String(), `"username":"ui-user"`)
	assert.Equal(t, csrfToken, sessionResp.Header().Get("X-CSRF-Token"))

	createBody := bytes.NewBufferString(`{"name":"ui-key","permissions":["read"]}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/ui/auth/api-keys", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.AddCookie(sessionCookie)
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)

	require.Equal(t, http.StatusCreated, createResp.Code)
	assert.Equal(t, "no-store", createResp.Header().Get("Cache-Control"))

	var createPayload struct {
		APIKey struct {
			ID string `json:"id"`
		} `json:"api_key"`
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &createPayload))
	require.NotEmpty(t, createPayload.APIKey.ID)
	require.NotEmpty(t, createPayload.Key)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/ui/auth/api-keys", nil)
	listReq.AddCookie(sessionCookie)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	require.Equal(t, http.StatusOK, listResp.Code)
	assert.Contains(t, listResp.Body.String(), `"name":"ui-key"`)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/ui/auth/api-keys/"+createPayload.APIKey.ID, nil)
	deleteReq.Header.Set("X-CSRF-Token", csrfToken)
	deleteReq.AddCookie(sessionCookie)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)

	require.Equal(t, http.StatusOK, deleteResp.Code)
	assert.Contains(t, deleteResp.Body.String(), "API key revoked successfully")
}
