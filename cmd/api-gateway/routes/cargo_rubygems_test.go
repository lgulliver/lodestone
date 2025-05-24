package routes

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestCargoRoutes_Setup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	api := router.Group("/api")
	
	// Create real services (they may fail internally, but we're testing route setup)
	// Create empty services just for route registration testing
	realRegistry := &registry.Service{}
	realAuth := &auth.Service{}
	
	// This tests that the route setup doesn't panic
	assert.NotPanics(t, func() {
		CargoRoutes(api, realRegistry, realAuth)
	})
	
	// Test that routes are registered by checking the gin router has routes
	routes := router.Routes()
	found := false
	for _, route := range routes {
		if strings.Contains(route.Path, "cargo") {
			found = true
			break
		}
	}
	assert.True(t, found, "Cargo routes should be registered")
}

func TestRubyGemsRoutes_Setup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	api := router.Group("/api")
	
	// Create empty services just for route registration testing
	realRegistry := &registry.Service{}
	realAuth := &auth.Service{}
	
	// This tests that the route setup doesn't panic
	assert.NotPanics(t, func() {
		RubyGemsRoutes(api, realRegistry, realAuth)
	})
	
	// Test that routes are registered by checking the gin router has routes
	routes := router.Routes()
	found := false
	for _, route := range routes {
		if strings.Contains(route.Path, "gems") {
			found = true
			break
		}
	}
	assert.True(t, found, "RubyGems routes should be registered")
}
