package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/lgulliver/lodestone/internal/auth"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/config"
)

func main() {
	// Load configuration
	cfg := config.LoadFromEnv()

	// Setup logging
	setupLogging(cfg.Logging)

	logrus.Info("Starting Lodestone API Gateway")

	// Initialize database
	db, err := common.NewDatabase(&cfg.Database)
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		logrus.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize cache
	cache, err := common.NewCache(&cfg.Redis)
	if err != nil {
		logrus.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer cache.Close()

	// Initialize storage
	storageFactory := storage.NewStorageFactory(&cfg.Storage)
	blobStorage, err := storageFactory.CreateStorage()
	if err != nil {
		logrus.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize services
	authService := auth.NewService(db, cache, &cfg.Auth)
	registryService := registry.NewService(db, blobStorage)

	// Setup HTTP server
	router := setupRouter(authService, registryService)
	
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logrus.Infof("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	} else {
		logrus.Info("Server shutdown complete")
	}
}

func setupLogging(cfg config.LoggingConfig) {
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	if cfg.Format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

func setupRouter(authService *auth.Service, registryService *registry.Service) *gin.Engine {
	// Set Gin mode based on environment
	if logrus.GetLevel() == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "lodestone-api-gateway",
			"time":    time.Now().UTC(),
		})
	})

	// API routes
	api := router.Group("/api/v1")
	{
		// Authentication routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", handleRegister(authService))
			auth.POST("/login", handleLogin(authService))
			auth.POST("/api-keys", authMiddleware(authService), handleCreateAPIKey(authService))
			auth.GET("/api-keys", authMiddleware(authService), handleListAPIKeys(authService))
			auth.DELETE("/api-keys/:id", authMiddleware(authService), handleRevokeAPIKey(authService))
		}

		// Registry routes
		registries := api.Group("/registries")
		registries.Use(authMiddleware(authService))
		{
			registries.GET("/:type/packages", handleListPackages(registryService))
			registries.POST("/:type/packages/:name/:version", handleUploadPackage(registryService))
			registries.GET("/:type/packages/:name/:version", handleDownloadPackage(registryService))
			registries.DELETE("/:type/packages/:name/:version", handleDeletePackage(registryService))
		}

		// Registry-specific endpoints
		setupRegistrySpecificRoutes(api, authService, registryService)
	}

	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// setupRegistrySpecificRoutes sets up registry-specific API endpoints
func setupRegistrySpecificRoutes(api *gin.RouterGroup, authService *auth.Service, registryService *registry.Service) {
	// NuGet specific routes
	nuget := api.Group("/nuget")
	nuget.Use(authMiddleware(authService))
	{
		nuget.GET("/v3/index.json", handleNuGetServiceIndex())
		nuget.PUT("/v2/package", handleNuGetUpload(registryService))
		nuget.GET("/v3/flatcontainer/:id/index.json", handleNuGetPackageVersions(registryService))
		nuget.GET("/v3/flatcontainer/:id/:version/:id.:version.nupkg", handleNuGetDownload(registryService))
	}

	// npm specific routes
	npm := api.Group("/npm")
	npm.Use(authMiddleware(authService))
	{
		npm.GET("/:name", handleNPMPackageInfo(registryService))
		npm.PUT("/:name", handleNPMPublish(registryService))
		npm.GET("/:name/-/:name-:version.tgz", handleNPMDownload(registryService))
	}

	// Go module proxy routes
	gomod := api.Group("/go")
	gomod.Use(authMiddleware(authService))
	{
		gomod.GET("/:module/@v/list", handleGoModuleVersions(registryService))
		gomod.GET("/:module/@v/:version.info", handleGoModuleInfo(registryService))
		gomod.GET("/:module/@v/:version.mod", handleGoModuleFile(registryService))
		gomod.GET("/:module/@v/:version.zip", handleGoModuleDownload(registryService))
	}

	// TODO: Add more registry-specific routes as needed
}
