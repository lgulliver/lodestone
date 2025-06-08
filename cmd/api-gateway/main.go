package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/lgulliver/lodestone/cmd/api-gateway/middleware"
	"github.com/lgulliver/lodestone/cmd/api-gateway/routes"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/config"
)

func main() {
	// Load configuration and set up logging
	cfg := config.LoadFromEnv()
	cfg.Logging.SetupLogging()

	// Initialize database connection
	database, err := common.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	// Database migrations should be run separately using the migrate command
	// This avoids conflicts between GORM AutoMigrate and SQL migrations
	log.Info().Msg("Database connection established - migrations managed separately")

	// Initialize cache (Redis)
	cache, err := common.NewCache(&cfg.Redis)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to connect to Redis cache, continuing without cache")
		cache = nil // Optional component
	}

	// Initialize storage backend using factory
	storageFactory := storage.NewStorageFactory(&cfg.Storage)
	storageBackend, err := storageFactory.CreateStorage()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}

	// Initialize services with database connections
	authService := auth.NewService(database, cache, &cfg.Auth)
	registryService := registry.NewService(database, storageBackend)

	// Initialize registry settings service for runtime control
	registrySettingsService := registry.NewRegistrySettingsService(database.DB)

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

	// Health check endpoint - support both GET and HEAD for Docker health checks
	healthHandler := func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "lodestone-api-gateway",
		})
	}
	router.GET("/health", healthHandler)
	router.HEAD("/health", healthHandler)

	// API routes
	api := router.Group("/api/v1")

	// Add registry validation middleware to all package format routes
	packageRoutes := api.Group("")
	packageRoutes.Use(middleware.RegistryValidationMiddleware(registrySettingsService))

	// Set up all package format routes with registry validation
	routes.AuthRoutes(api, authService)
	routes.AdminRoutes(api, registryService, authService) // Admin routes without registry validation
	routes.PackageOwnershipRoutes(api, registryService, authService)
	routes.NuGetRoutes(packageRoutes, registryService, authService)
	routes.NPMRoutes(packageRoutes, registryService, authService)
	routes.MavenRoutes(packageRoutes, registryService, authService)
	routes.GoRoutes(packageRoutes, registryService, authService)
	routes.HelmRoutes(packageRoutes, registryService, authService)
	routes.CargoRoutes(packageRoutes, registryService, authService)
	routes.RubyGemsRoutes(packageRoutes, registryService, authService)
	routes.OPARoutes(packageRoutes, registryService, authService)

	// OCI/Docker registry routes need to be at root level for Docker CLI compatibility
	routes.OCIRootRoutes(router, registryService, authService)

	// Start server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Str("address", serverAddr).
		Msg("Starting Lodestone API Gateway")

	if err := router.Run(serverAddr); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
