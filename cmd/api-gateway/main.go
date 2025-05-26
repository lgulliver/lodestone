package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

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

	// Initialize storage backend
	storageBackend, err := storage.NewLocalStorage("./storage")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}

	// Initialize services with database connections
	authService := auth.NewService(database, cache, &cfg.Auth)
	registryService := registry.NewService(database, storageBackend)

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
	routes.MavenRoutes(api, registryService, authService)
	routes.GoRoutes(api, registryService, authService)
	routes.HelmRoutes(api, registryService, authService)
	routes.CargoRoutes(api, registryService, authService)
	routes.RubyGemsRoutes(api, registryService, authService)
	routes.OPARoutes(api, registryService, authService)

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
