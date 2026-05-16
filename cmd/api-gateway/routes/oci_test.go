package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestOCIRootRoutes verifies that OCI root routes can be registered without panicking
func TestOCIRootRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Create empty services just for route registration testing
	realRegistry := &registry.Service{}
	realAuth := &auth.Service{}

	// This tests that the route setup doesn't panic
	assert.NotPanics(t, func() {
		OCIRootRoutes(router, realRegistry, realAuth)
	})

	// Test that catch-all route is registered
	routes := router.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/v2/*path" {
			found = true
			break
		}
	}
	assert.True(t, found, "OCI catch-all route should be registered")
}

// TestOCIBaseEndpoint tests the base endpoint handler directly
func TestOCIBaseEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/v2/", handleOCIBase())

	req := httptest.NewRequest("GET", "/v2/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lodestone OCI Registry")
	assert.Equal(t, "registry/2.0", w.Header().Get("Docker-Distribution-API-Version"))
}

// TestExtractRepositoryNameFunction tests the repository name extraction helper
func TestExtractRepositoryNameFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		param    string
		expected string
	}{
		{
			name:     "Name with leading slash",
			param:    "/test-repo",
			expected: "test-repo",
		},
		{
			name:     "Name without leading slash",
			param:    "test-repo",
			expected: "test-repo",
		},
		{
			name:     "Empty parameter",
			param:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test gin context with the parameter
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Params = gin.Params{{Key: "name", Value: tt.param}}

			result := extractRepositoryName(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOCIBaseEndpointOnly tests only the base endpoint that doesn't require database access
func TestOCIBaseEndpointOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Create real services for the handler
	realRegistry := &registry.Service{}
	realAuth := &auth.Service{}

	OCIRootRoutes(router, realRegistry, realAuth)

	req := httptest.NewRequest("GET", "/v2/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Only the base endpoint works without database dependencies
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lodestone OCI Registry")
	assert.Equal(t, "registry/2.0", w.Header().Get("Docker-Distribution-API-Version"))
}

type ociRouteTestStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newOCIRouteTestStorage() *ociRouteTestStorage {
	return &ociRouteTestStorage{data: make(map[string][]byte)}
}

func (s *ociRouteTestStorage) Store(_ context.Context, path string, r io.Reader, _ string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.data[path] = b
	s.mu.Unlock()
	return nil
}

func (s *ociRouteTestStorage) Retrieve(_ context.Context, path string) (io.ReadCloser, error) {
	s.mu.Lock()
	b, ok := s.data[path]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (s *ociRouteTestStorage) Delete(_ context.Context, path string) error {
	s.mu.Lock()
	delete(s.data, path)
	s.mu.Unlock()
	return nil
}

func (s *ociRouteTestStorage) Exists(_ context.Context, path string) (bool, error) {
	s.mu.Lock()
	_, ok := s.data[path]
	s.mu.Unlock()
	return ok, nil
}

func (s *ociRouteTestStorage) GetSize(_ context.Context, path string) (int64, error) {
	s.mu.Lock()
	b, ok := s.data[path]
	s.mu.Unlock()
	if !ok {
		return 0, fmt.Errorf("not found: %s", path)
	}
	return int64(len(b)), nil
}

func (s *ociRouteTestStorage) List(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func setupOCIOwnershipRouteTest(t *testing.T) (*registry.Service, *types.User, *types.User) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.User{}, &types.Artifact{}, &types.PackageOwnership{}, &types.RegistrySetting{}))

	registryService := registry.NewService(&common.Database{DB: db}, newOCIRouteTestStorage())

	owner := &types.User{Username: "owner", Email: "owner@example.com", Password: "pw", IsActive: true}
	outsider := &types.User{Username: "outsider", Email: "outsider@example.com", Password: "pw", IsActive: true}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(outsider).Error)
	require.NoError(t, registryService.Ownership.EstablishInitialOwnership(context.Background(), "oci", "repo", owner.ID))

	return registryService, owner, outsider
}

func TestOCIManifestPutForbiddenForNonOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registryService, _, outsider := setupOCIOwnershipRouteTest(t)

	router := gin.New()
	router.PUT("/v2/:name/manifests/:reference", func(c *gin.Context) {
		c.Set("user", outsider)
		handleOCIManifestPut(registryService)(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/v2/repo/manifests/latest", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient permissions")
}

func TestOCIBlobUploadStartForbiddenForNonOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registryService, _, outsider := setupOCIOwnershipRouteTest(t)

	router := gin.New()
	router.POST("/v2/:name/blobs/uploads/", func(c *gin.Context) {
		c.Set("user", outsider)
		handleOCIBlobUploadStart(registryService)(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v2/repo/blobs/uploads/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient permissions")
}

func TestOCIBlobDeleteForbiddenForNonOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registryService, _, outsider := setupOCIOwnershipRouteTest(t)

	router := gin.New()
	router.DELETE("/v2/:name/blobs/:digest", func(c *gin.Context) {
		c.Set("user", outsider)
		handleOCIBlobDelete(registryService)(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/v2/repo/blobs/sha256:abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient permissions")
}

func setupOCITokenAuthService(t *testing.T, permissions []string) (*auth.Service, string, *types.User) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.User{}, &types.APIKey{}))

	authService := auth.NewService(&common.Database{DB: db}, nil, &config.AuthConfig{
		JWTSecret:     "oci-token-test-secret",
		JWTExpiration: time.Hour,
		BCryptCost:    4,
	})

	user, err := authService.Register(context.Background(), &types.RegisterRequest{
		Username: "oci-user",
		Email:    "oci@example.com",
		Password: "test-password",
	})
	require.NoError(t, err)

	_, apiKey, err := authService.CreateAPIKey(context.Background(), user.ID, "oci-test-key", permissions)
	require.NoError(t, err)

	return authService, apiKey, user
}

func TestOCIBlobUploadStartAllowedForOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registryService, owner, _ := setupOCIOwnershipRouteTest(t)

	router := gin.New()
	router.POST("/v2/:name/blobs/uploads/", func(c *gin.Context) {
		c.Set("user", owner)
		handleOCIBlobUploadStart(registryService)(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v2/repo/blobs/uploads/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.NotEmpty(t, w.Header().Get("Location"))
	assert.NotEmpty(t, w.Header().Get("Docker-Upload-UUID"))
}

func TestOCIManifestPutAllowedForOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registryService, owner, _ := setupOCIOwnershipRouteTest(t)

	router := gin.New()
	router.PUT("/v2/:name/manifests/:reference", func(c *gin.Context) {
		c.Set("user", owner)
		handleOCIManifestPut(registryService)(c)
	})

	manifest := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[]}`
	req := httptest.NewRequest(http.MethodPut, "/v2/repo/manifests/latest", strings.NewReader(manifest))
	req.Header.Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotEmpty(t, w.Header().Get("Docker-Content-Digest"))
}

func TestOCIManifestGetAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registryService, owner, _ := setupOCIOwnershipRouteTest(t)

	// First, put a manifest so we can retrieve it
	router := gin.New()
	router.PUT("/v2/:name/manifests/:reference", func(c *gin.Context) {
		c.Set("user", owner)
		handleOCIManifestPut(registryService)(c)
	})
	router.GET("/v2/:name/manifests/:reference", func(c *gin.Context) {
		c.Set("user", owner)
		handleOCIManifestGet(registryService)(c)
	})

	manifest := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[]}`
	putReq := httptest.NewRequest(http.MethodPut, "/v2/repo/manifests/v1.0", strings.NewReader(manifest))
	putReq.Header.Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	putW := httptest.NewRecorder()
	router.ServeHTTP(putW, putReq)
	require.Equal(t, http.StatusCreated, putW.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/v2/repo/manifests/v1.0", nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)

	assert.Equal(t, http.StatusOK, getW.Code)
	assert.NotEmpty(t, getW.Header().Get("Docker-Content-Digest"))
}

func TestDockerTokenIssuesScopedJWTInsteadOfAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authService, apiKey, user := setupOCITokenAuthService(t, []string{"read", "write"})

	router := gin.New()
	router.GET("/v2/token", handleDockerToken(authService))

	req := httptest.NewRequest(http.MethodGet, "/v2/token?service=registry&scope=repository:repo:pull", nil)
	req.SetBasicAuth("ignored", apiKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), apiKey)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	issuedToken, ok := response["token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, issuedToken)

	tokenUser, claims, err := authService.ValidateOCIToken(context.Background(), issuedToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, tokenUser.ID)
	assert.Equal(t, "registry", claims.Service)
	assert.Equal(t, []string{"repository:repo:pull"}, claims.Scope)
}

func TestDockerTokenDeniesScopeWhenAPIKeyLacksPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authService, apiKey, _ := setupOCITokenAuthService(t, []string{"read"})

	router := gin.New()
	router.GET("/v2/token", handleDockerToken(authService))

	req := httptest.NewRequest(http.MethodGet, "/v2/token?service=registry&scope=repository:repo:push", nil)
	req.SetBasicAuth("ignored", apiKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient permissions")
}
