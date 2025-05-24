package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	
	"github.com/lgulliver/lodestone/cmd/api-gateway/routes"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
)

func main() {
	// Set up logging
	logrus.SetLevel(logrus.InfoLevel)
	if os.Getenv("DEBUG") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Initialize storage backend
	storageBackend := storage.NewLocal("./storage")

	// Initialize services
	authService := auth.NewService(nil, nil, nil) // TODO: Add database, cache, and config when implemented
	registryService := registry.NewService(storageBackend)

	// Set up Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"service": "lodestone-api-gateway",
		})
	})

	// API routes
	api := router.Group("/api/v1")
	
	// Set up all package format routes
	routes.AuthRoutes(api, authService)
	routes.NuGetRoutes(api, registryService, authService)
	routes.NPMRoutes(api, registryService, authService)
	routes.OCIRoutes(api, registryService, authService)
	routes.MavenRoutes(api, registryService, authService)
	routes.GoRoutes(api, registryService, authService)
	routes.HelmRoutes(api, registryService, authService)
	routes.CargoRoutes(api, registryService, authService)
	routes.RubyGemsRoutes(api, registryService, authService)
	routes.OPARoutes(api, registryService, authService)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logrus.Infof("Starting Lodestone API Gateway on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
