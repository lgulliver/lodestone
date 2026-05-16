package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExtractRegistryType_PathParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "API v1 npm path",
			path:     "/api/v1/npm/test-package",
			expected: "npm",
		},
		{
			name:     "API v1 rubygems path via gems prefix",
			path:     "/api/v1/gems/api/v1/gems",
			expected: "rubygems",
		},
		{
			name:     "API v1 OCI path",
			path:     "/api/v1/v2/library/nginx/manifests/latest",
			expected: "oci",
		},
		{
			name:     "Root OCI path",
			path:     "/v2/library/nginx/manifests/latest",
			expected: "oci",
		},
		{
			name:     "Unknown API path",
			path:     "/api/v1/unknown/endpoint",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)
			assert.Equal(t, tt.expected, extractRegistryType(c))
		})
	}
}

func TestRegistryValidationMiddleware_DisabledRegistryReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsService, db := newTestRegistrySettingsService(t)
	seedRegistrySetting(t, db, "npm", false)

	router := gin.New()
	router.Use(RegistryValidationMiddleware(settingsService))
	router.GET("/api/v1/npm/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/npm/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Registry is currently disabled", body["error"])
	assert.Equal(t, "npm", body["registry"])
}

func TestRegistryValidationMiddleware_RootOCIPathUsesOCIRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsService, db := newTestRegistrySettingsService(t)
	seedRegistrySetting(t, db, "oci", false)

	router := gin.New()
	router.Use(RegistryValidationMiddleware(settingsService))
	router.GET("/v2/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/v2/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "oci", body["registry"])
}

func TestRegistryValidationMiddleware_DatabaseErrorReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsService, db := newTestRegistrySettingsService(t)
	require.NoError(t, db.Exec("DROP TABLE registry_settings").Error)

	router := gin.New()
	router.Use(RegistryValidationMiddleware(settingsService))
	router.GET("/api/v1/npm/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/npm/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Internal server error", body["error"])
}

func newTestRegistrySettingsService(t *testing.T) (*registry.RegistrySettingsService, *gorm.DB) {
	t.Helper()

	dsn := "file:registry-validation-" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.RegistrySetting{}))

	return registry.NewRegistrySettingsService(db), db
}

func seedRegistrySetting(t *testing.T, db *gorm.DB, registryName string, enabled bool) {
	t.Helper()

	require.NoError(t, db.Model(&types.RegistrySetting{}).Create(map[string]any{
		"id":            uuid.New(),
		"registry_name": registryName,
		"enabled":       enabled,
		"description":   "test setting",
	}).Error)
}
