package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/lgulliver/lodestone/cmd/api-gateway/routes"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/config"
)

func main() {
	// Load configuration and set up logging
	cfg := config.LoadFromEnv()
	cfg.Logging.SetupLogging()

	// Initialize storage backend
	storageBackend, err := storage.NewLocalStorage("./storage")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}

	// Initialize services
	authService := auth.NewService(nil, nil, nil)               // TODO: Add database, cache, and config when implemented
	registryService := registry.NewService(nil, storageBackend) // TODO: Add database when implemented

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
			"status":  "healthy",
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

	log.Info().Str("port", port).Msg("Starting Lodestone API Gateway")
	if err := router.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
