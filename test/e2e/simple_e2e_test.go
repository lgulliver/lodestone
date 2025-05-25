// filepath: /home/liam/repos/lodestone/test/e2e/simple_e2e_test.go
// Simple end-to-end test to verify enhanced storage with registry service
package e2e_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSimpleE2EWorkflow(t *testing.T) {
	t.Log("=== Simple Enhanced Storage E2E Test ===")

	// Create temporary directory for test data
	testDir, err := os.MkdirTemp("", "lodestone-simple-e2e")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(testDir)

	// Setup test environment
	service := setupSimpleTestEnvironment(t, testDir)

	// Run basic workflow test
	t.Log("Testing simple upload/download workflow...")
	testSimpleWorkflow(t, service)

	t.Log("✅ Simple E2E test completed successfully!")
}

func setupSimpleTestEnvironment(t *testing.T, testDir string) *registry.Service {
	// Setup in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal("Failed to connect to database:", err)
	}

	// Create basic tables
	err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password TEXT NOT NULL,
			is_active BOOLEAN DEFAULT true,
			is_admin BOOLEAN DEFAULT false,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatal("Failed to create users table:", err)
	}

	err = db.Exec(`
		CREATE TABLE artifacts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			registry TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			content_type TEXT,
			size INTEGER,
			sha256 TEXT,
			metadata TEXT,
			downloads INTEGER DEFAULT 0,
			published_by TEXT NOT NULL,
			is_public BOOLEAN DEFAULT false,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatal("Failed to create artifacts table:", err)
	}

	// Setup storage
	storageDir := filepath.Join(testDir, "storage")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatal("Failed to create storage directory:", err)
	}

	storageBackend, err := storage.NewLocalStorage(storageDir)
	if err != nil {
		t.Fatal("Failed to create local storage:", err)
	}

	// Create database wrapper and service
	commonDB := &common.Database{DB: db}
	service := registry.NewService(commonDB, storageBackend)

	return service
}

func testSimpleWorkflow(t *testing.T, service *registry.Service) {
	ctx := context.Background()
	userID := uuid.New()

	// Test data
	packageName := "simple-test-package"
	version := "1.0.0"
	content := `{"name": "simple-test-package", "version": "1.0.0", "main": "index.js"}`

	t.Log("1. Uploading test package...")

	// Upload
	artifact, err := service.Upload(ctx, "npm", packageName, version, strings.NewReader(content), userID)
	if err != nil {
		t.Fatal("Upload failed:", err)
	}

	t.Logf("✅ Uploaded: %s@%s (Size: %d bytes, SHA256: %s)",
		artifact.Name, artifact.Version, artifact.Size, artifact.SHA256)

	t.Log("2. Downloading test package...")

	// Download
	downloadedArtifact, reader, err := service.Download(ctx, "npm", packageName, version)
	if err != nil {
		t.Fatal("Download failed:", err)
	}
	defer reader.Close()

	downloadedContent, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal("Failed to read downloaded content:", err)
	}

	// Verify content matches
	if string(downloadedContent) != content {
		t.Fatalf("Content mismatch:\nExpected: %s\nGot: %s", content, string(downloadedContent))
	}

	// Verify metadata matches
	if downloadedArtifact.Name != packageName || downloadedArtifact.Version != version {
		t.Fatalf("Metadata mismatch: expected %s@%s, got %s@%s",
			packageName, version, downloadedArtifact.Name, downloadedArtifact.Version)
	}

	t.Logf("✅ Downloaded: %s@%s (verified content integrity)",
		downloadedArtifact.Name, downloadedArtifact.Version)

	t.Log("3. Testing list functionality...")

	// Test list
	filter := &types.ArtifactFilter{
		Name:     packageName,
		Registry: "npm",
	}

	artifacts, _, err := service.List(ctx, filter)
	if err != nil {
		t.Fatal("List failed:", err)
	}

	found := false
	for _, art := range artifacts {
		if art.Name == packageName && art.Version == version {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Uploaded package not found in list results")
	}

	t.Logf("✅ Listed packages: found %d total packages, including our test package", len(artifacts))
}
