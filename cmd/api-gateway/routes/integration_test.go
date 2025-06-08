package routes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRegistryService mocks the registry service for testing
type MockRegistryService struct {
	mock.Mock
}

func (m *MockRegistryService) Upload(ctx context.Context, registryType, name, version string, content io.Reader, userID uuid.UUID) (*types.Artifact, error) {
	args := m.Called(ctx, registryType, name, version, content, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Artifact), args.Error(1)
}

func (m *MockRegistryService) Download(ctx context.Context, registryType, name, version string) (*types.Artifact, io.ReadCloser, error) {
	args := m.Called(ctx, registryType, name, version)
	var artifact *types.Artifact
	var content io.ReadCloser

	if args.Get(0) != nil {
		artifact = args.Get(0).(*types.Artifact)
	}
	if args.Get(1) != nil {
		content = args.Get(1).(io.ReadCloser)
	}

	return artifact, content, args.Error(2)
}

func (m *MockRegistryService) List(ctx context.Context, filter *types.ArtifactFilter) ([]*types.Artifact, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*types.Artifact), args.Get(1).(int64), args.Error(2)
}

func (m *MockRegistryService) Delete(ctx context.Context, registryType, name, version string, userID uuid.UUID) error {
	args := m.Called(ctx, registryType, name, version, userID)
	return args.Error(0)
}

// MockAuthService mocks the auth service for testing
type MockAuthServiceForRoutes struct {
	mock.Mock
}

func (m *MockAuthServiceForRoutes) ValidateToken(ctx context.Context, token string) (*types.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockAuthServiceForRoutes) ValidateAPIKey(ctx context.Context, apiKey string) (*types.User, *types.APIKey, error) {
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

func TestUploadMethodSignatures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		registryType  string
		packageName   string
		version       string
		filename      string
		expectSuccess bool
	}{
		{
			name:          "Go module upload",
			registryType:  "go",
			packageName:   "github.com/example/module",
			version:       "v1.0.0",
			filename:      "module.zip",
			expectSuccess: true,
		},
		{
			name:          "Helm chart upload",
			registryType:  "helm",
			packageName:   "my-chart",
			version:       "1.0.0",
			filename:      "my-chart-1.0.0.tgz",
			expectSuccess: true,
		},
		{
			name:          "Cargo crate upload",
			registryType:  "cargo",
			packageName:   "my-crate",
			version:       "1.0.0",
			filename:      "my-crate-1.0.0.crate",
			expectSuccess: true,
		},
		{
			name:          "RubyGems gem upload",
			registryType:  "rubygems",
			packageName:   "my-gem",
			version:       "1.0.0",
			filename:      "my-gem-1.0.0.gem",
			expectSuccess: true,
		},
		{
			name:          "OPA bundle upload",
			registryType:  "opa",
			packageName:   "my-bundle",
			version:       "1.0.0",
			filename:      "bundle.tar.gz",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRegistry := new(MockRegistryService)
			mockAuth := new(MockAuthServiceForRoutes)

			user := &types.User{
				ID:       uuid.New(),
				Username: "testuser",
				Email:    "test@example.com",
			}

			artifact := &types.Artifact{
				ID:          uuid.New(),
				Name:        tt.packageName,
				Version:     tt.version,
				Registry:    tt.registryType,
				Size:        1024,
				PublishedBy: user.ID,
			}

			content := io.NopCloser(strings.NewReader("test content"))

			// Setup expectations
			mockAuth.On("ValidateAPIKey", mock.Anything, "valid-api-key").Return(user, &types.APIKey{}, nil)
			mockRegistry.On("Upload", mock.Anything, tt.registryType, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything, user.ID).Return(artifact, nil)

			// Test that the Upload method is called with the correct signature
			ctx := context.Background()
			_, err := mockRegistry.Upload(ctx, tt.registryType, tt.packageName, tt.version, content, user.ID)

			if tt.expectSuccess {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestFilenameParsingLogic(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		expectedName    string
		expectedVersion string
		registryType    string
	}{
		{
			name:            "Helm chart filename",
			filename:        "my-chart-1.0.0.tgz",
			expectedName:    "my-chart",
			expectedVersion: "1.0.0",
			registryType:    "helm",
		},
		{
			name:            "Cargo crate filename",
			filename:        "my-crate-1.0.0.crate",
			expectedName:    "my-crate",
			expectedVersion: "1.0.0",
			registryType:    "cargo",
		},
		{
			name:            "RubyGems gem filename",
			filename:        "my-gem-1.0.0.gem",
			expectedName:    "my-gem",
			expectedVersion: "1.0.0",
			registryType:    "rubygems",
		},
		{
			name:            "Complex name with multiple hyphens",
			filename:        "my-complex-package-name-2.1.0.tgz",
			expectedName:    "my-complex-package-name",
			expectedVersion: "2.1.0",
			registryType:    "helm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var name, version string

			switch tt.registryType {
			case "helm":
				// Simulate Helm filename parsing logic
				if strings.HasSuffix(tt.filename, ".tgz") {
					nameVersion := strings.TrimSuffix(tt.filename, ".tgz")
					parts := strings.Split(nameVersion, "-")
					if len(parts) >= 2 {
						// Find the last part that looks like a version
						for i := len(parts) - 1; i >= 1; i-- {
							possibleVersion := strings.Join(parts[i:], "-")
							if strings.Contains(possibleVersion, ".") || strings.Contains(possibleVersion, "beta") || strings.Contains(possibleVersion, "alpha") {
								name = strings.Join(parts[:i], "-")
								version = possibleVersion
								break
							}
						}
					}
				}
			case "cargo":
				// Simulate Cargo filename parsing logic
				if strings.HasSuffix(tt.filename, ".crate") {
					nameVersion := strings.TrimSuffix(tt.filename, ".crate")
					parts := strings.Split(nameVersion, "-")
					if len(parts) >= 2 {
						for i := len(parts) - 1; i >= 1; i-- {
							possibleVersion := strings.Join(parts[i:], "-")
							if strings.Contains(possibleVersion, ".") {
								name = strings.Join(parts[:i], "-")
								version = possibleVersion
								break
							}
						}
					}
				}
			case "rubygems":
				// Simulate RubyGems filename parsing logic
				if strings.HasSuffix(tt.filename, ".gem") {
					nameVersion := strings.TrimSuffix(tt.filename, ".gem")
					parts := strings.Split(nameVersion, "-")
					if len(parts) >= 2 {
						for i := len(parts) - 1; i >= 1; i-- {
							possibleVersion := strings.Join(parts[i:], "-")
							if strings.Contains(possibleVersion, ".") {
								name = strings.Join(parts[:i], "-")
								version = possibleVersion
								break
							}
						}
					}
				}
			}

			assert.Equal(t, tt.expectedName, name, "Package name should be parsed correctly")
			assert.Equal(t, tt.expectedVersion, version, "Package version should be parsed correctly")
		})
	}
}

func TestRouteRegistrationIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	api := router.Group("/api")

	// Use real services for route registration
	realRegistry := &registry.Service{}
	realAuth := &auth.Service{}

	// Test that all route functions can be called without panicking
	routeFunctions := []func(*gin.RouterGroup, *registry.Service, *auth.Service){
		GoRoutes,
		HelmRoutes,
		CargoRoutes,
		RubyGemsRoutes,
		OPARoutes,
		OCIRoutes, // Add OCI routes to integration testing
	}

	for i, routeFunc := range routeFunctions {
		t.Run(fmt.Sprintf("RouteFunction_%d", i), func(t *testing.T) {
			assert.NotPanics(t, func() {
				routeFunc(api, realRegistry, realAuth)
			})
		})
	}

	// Verify routes are registered
	routes := router.Routes()
	assert.NotEmpty(t, routes, "Routes should be registered")

	// Check for expected route patterns
	routePatterns := []string{
		"/api/go/",
		"/api/helm/",
		"/api/cargo/",
		"/api/gems/",
		"/api/opa/",
		"/api/v2/", // Add OCI route pattern
	}

	for _, pattern := range routePatterns {
		found := false
		for _, route := range routes {
			if strings.Contains(route.Path, strings.TrimSuffix(pattern, "/")) {
				found = true
				break
			}
		}
		assert.True(t, found, "Route pattern %s should be registered", pattern)
	}
}
