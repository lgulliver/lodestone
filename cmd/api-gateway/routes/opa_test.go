package routes

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestOPARoutes_Setup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	api := router.Group("/api")
	
	// Create empty services just for route registration testing
	realRegistry := &registry.Service{}
	realAuth := &auth.Service{}
	
	// This tests that the route setup doesn't panic
	assert.NotPanics(t, func() {
		OPARoutes(api, realRegistry, realAuth)
	})
	
	// Test that routes are registered by checking the gin router has routes
	routes := router.Routes()
	found := false
	for _, route := range routes {
		if strings.Contains(route.Path, "opa") {
			found = true
			break
		}
	}
	assert.True(t, found, "OPA routes should be registered")
}
