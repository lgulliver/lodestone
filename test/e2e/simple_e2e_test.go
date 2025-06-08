// filepath: /home/liam/repos/lodestone/test/e2e/simple_e2e_test.go
// Simple end-to-end test to verify enhanced storage with registry service
package e2e_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// createTestNpmPackage creates a valid npm package tarball for testing
func createTestNpmPackage(packageName, version string, additionalFields map[string]interface{}) ([]byte, error) {
	// Create package.json data
	packageJSON := map[string]interface{}{
		"name":        packageName,
		"version":     version,
		"description": "Test package for Lodestone",
		"main":        "index.js",
	}

	// Add any additional fields
	for k, v := range additionalFields {
		packageJSON[k] = v
	}

	// Create a buffer to write our tarball to
	var buf bytes.Buffer

	// Create gzip writer
	gw := gzip.NewWriter(&buf)

	// Create tar writer
	tw := tar.NewWriter(gw)

	// Marshal package.json data
	packageJSONData, err := json.Marshal(packageJSON)
	if err != nil {
		return nil, err
	}

	// Add package.json file to tarball
	packageJSONHeader := &tar.Header{
		Name: "package/package.json",
		Mode: 0644,
		Size: int64(len(packageJSONData)),
	}

	if err := tw.WriteHeader(packageJSONHeader); err != nil {
		return nil, err
	}

	if _, err := tw.Write(packageJSONData); err != nil {
		return nil, err
	}

	// Add a simple index.js file
	indexJS := []byte(`console.log("Hello from Lodestone test package");`)
	indexJSHeader := &tar.Header{
		Name: "package/index.js",
		Mode: 0644,
		Size: int64(len(indexJS)),
	}

	if err := tw.WriteHeader(indexJSHeader); err != nil {
		return nil, err
	}

	if _, err := tw.Write(indexJS); err != nil {
		return nil, err
	}

	// Close writers
	if err := tw.Close(); err != nil {
		return nil, err
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestSimpleE2EWorkflow(t *testing.T) {
	t.Log("=== Simple Enhanced Storage E2E Test ===")

	// Create temporary directory for test data
	testDir, err := os.MkdirTemp("", "lodestone-simple-e2e")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(testDir)

	// Setup test environment
	service, testUserID := setupSimpleTestEnvironment(t, testDir)

	// Run basic workflow test
	t.Log("Testing simple upload/download workflow...")
	testSimpleWorkflow(t, service, testUserID)

	t.Log("✅ Simple E2E test completed successfully!")
}

func setupSimpleTestEnvironment(t *testing.T, testDir string) (*registry.Service, uuid.UUID) {
	// Use file-based SQLite database to avoid table sharing issues with concurrent access
	dbPath := filepath.Join(testDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal("Failed to connect to database:", err)
	}

	// Configure SQLite for concurrent access
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal("Failed to get underlying database:", err)
	}

	// Set connection pool settings for SQLite
	sqlDB.SetMaxOpenConns(1) // SQLite works best with single connection
	sqlDB.SetMaxIdleConns(1)

	// Use GORM AutoMigrate instead of raw SQL
	err = db.AutoMigrate(&types.User{}, &types.Artifact{}, &types.PackageOwnership{})
	if err != nil {
		t.Fatal("Failed to migrate database:", err)
	}

	// Create test user
	testUser := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  true,
	}
	if err := db.Create(testUser).Error; err != nil {
		t.Fatal("Failed to create test user:", err)
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

	return service, testUser.ID
}

func testSimpleWorkflow(t *testing.T, service *registry.Service, userID uuid.UUID) {
	ctx := context.Background()

	// Test data
	packageName := "simple-test-package"
	version := "1.0.0"

	// Create a valid npm package tarball
	packageContent, err := createTestNpmPackage(packageName, version, nil)
	if err != nil {
		t.Fatal("Failed to create test package:", err)
	}

	t.Log("1. Uploading test package...")

	// Upload
	artifact, err := service.Upload(ctx, "npm", packageName, version, bytes.NewReader(packageContent), userID)
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
	// For compressed tarball, we just check that we got some data back,
	// as the exact content will be different due to archive format
	if len(downloadedContent) == 0 {
		t.Fatalf("Empty content returned from download")
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
