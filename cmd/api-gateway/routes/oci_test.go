package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/stretchr/testify/assert"
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
