// filepath: /home/liam/repos/lodestone/test/storage/storage_registry_integration_test.go
package storage_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/registry"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// createTestNpmTarball creates a properly formatted npm package tarball
func createTestNpmTarball(packageJSON string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Parse the package.json to get name
	var pkgData map[string]interface{}
	if err := json.Unmarshal([]byte(packageJSON), &pkgData); err != nil {
		return nil, err
	}

	// Add package.json file to tarball
	packageJSONHeader := &tar.Header{
		Name:    "package/package.json",
		Mode:    0644,
		Size:    int64(len(packageJSON)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(packageJSONHeader); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(packageJSON)); err != nil {
		return nil, err
	}

	// Add a simple index.js file
	indexJS := []byte(`console.log("Hello from test package");`)
	indexJSHeader := &tar.Header{
		Name:    "package/index.js",
		Mode:    0644,
		Size:    int64(len(indexJS)),
		ModTime: time.Now(),
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

func TestStorageIntegrationWithRegistry(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run auto migrations
	err = db.AutoMigrate(&types.User{}, &types.APIKey{}, &types.Artifact{})
	require.NoError(t, err)

	commonDB := &common.Database{DB: db}

	// Setup test storage
	tempDir := t.TempDir()
	storageInstance, err := storage.NewLocalStorage(tempDir)
	require.NoError(t, err)

	// Create registry service
	registryService := registry.NewService(commonDB, storageInstance)

	// Create test user
	user := &types.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  false,
	}
	err = db.Create(user).Error
	require.NoError(t, err)

	// Test data
	ctx := context.Background()
	registryType := "npm"
	packageName := "test-package"
	version := "1.0.0"
	packageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"description": "A test npm package",
		"main": "index.js"
	}`
	userID := user.ID

	t.Run("upload and download workflow", func(t *testing.T) {
		// Create proper npm package tarball
		tarballData, err := createTestNpmTarball(packageJSON)
		require.NoError(t, err)

		// Upload artifact
		artifact, err := registryService.Upload(
			ctx,
			registryType,
			packageName,
			version,
			bytes.NewReader(tarballData),
			userID,
		)

		require.NoError(t, err)
		require.NotNil(t, artifact)
		assert.Equal(t, packageName, artifact.Name)
		assert.Equal(t, version, artifact.Version)
		assert.Equal(t, registryType, artifact.Registry)
		assert.Equal(t, userID, artifact.PublishedBy)
		assert.Greater(t, artifact.Size, int64(0))
		assert.NotEmpty(t, artifact.SHA256)
		assert.NotEmpty(t, artifact.StoragePath)

		// Verify file exists in storage
		exists, err := storageInstance.Exists(ctx, artifact.StoragePath)
		require.NoError(t, err)
		assert.True(t, exists)

		// Verify file size matches
		size, err := storageInstance.GetSize(ctx, artifact.StoragePath)
		require.NoError(t, err)
		assert.Equal(t, artifact.Size, size)

		// Download artifact
		downloadedArtifact, reader, err := registryService.Download(
			ctx,
			registryType,
			packageName,
			version,
		)

		require.NoError(t, err)
		require.NotNil(t, downloadedArtifact)
		require.NotNil(t, reader)
		defer reader.Close()

		// Verify downloaded artifact metadata
		assert.Equal(t, artifact.ID, downloadedArtifact.ID)
		assert.Equal(t, artifact.Name, downloadedArtifact.Name)
		assert.Equal(t, artifact.Version, downloadedArtifact.Version)
		assert.Equal(t, artifact.Registry, downloadedArtifact.Registry)

		// Verify downloaded content size
		downloadedContent, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, len(tarballData), len(downloadedContent))
	})

	t.Run("delete workflow", func(t *testing.T) {
		// Upload another artifact for deletion test
		deletePackageName := "delete-test-package"
		deleteVersion := "1.0.0"
		deletePackageJSON := `{"name": "delete-test-package", "version": "1.0.0"}`

		// Create proper npm package tarball
		tarballData, err := createTestNpmTarball(deletePackageJSON)
		require.NoError(t, err)

		artifact, err := registryService.Upload(
			ctx,
			registryType,
			deletePackageName,
			deleteVersion,
			bytes.NewReader(tarballData),
			userID,
		)
		require.NoError(t, err)

		// Verify file exists
		exists, err := storageInstance.Exists(ctx, artifact.StoragePath)
		require.NoError(t, err)
		assert.True(t, exists)

		// Delete artifact
		err = registryService.Delete(ctx, registryType, deletePackageName, deleteVersion, userID)
		require.NoError(t, err)

		// Verify file no longer exists
		exists, err = storageInstance.Exists(ctx, artifact.StoragePath)
		require.NoError(t, err)
		assert.False(t, exists)

		// Verify download fails
		_, _, err = registryService.Download(ctx, registryType, deletePackageName, deleteVersion)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "artifact not found")
	})

	t.Run("list files in storage", func(t *testing.T) {
		// Upload multiple packages to test listing
		packages := []struct {
			name    string
			version string
		}{
			{"list-test-package-1", "1.0.0"},
			{"list-test-package-2", "1.0.0"},
			{"list-test-package-1", "2.0.0"},
		}

		var uploadedPaths []string
		for _, pkg := range packages {
			// Create proper npm package for each test
			packageJSON := `{"name": "` + pkg.name + `", "version": "` + pkg.version + `"}`
			tarballData, err := createTestNpmTarball(packageJSON)
			require.NoError(t, err)

			artifact, err := registryService.Upload(
				ctx,
				registryType,
				pkg.name,
				pkg.version,
				bytes.NewReader(tarballData),
				userID,
			)
			require.NoError(t, err)
			uploadedPaths = append(uploadedPaths, artifact.StoragePath)
		}

		// List files with npm prefix
		files, err := storageInstance.List(ctx, "npm")
		require.NoError(t, err)

		// Verify all uploaded files are in the list
		for _, uploadedPath := range uploadedPaths {
			assert.Contains(t, files, uploadedPath)
		}
	})
}

func TestStorageFactoryIntegrationWithRegistry(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run auto migrations
	err = db.AutoMigrate(&types.User{}, &types.APIKey{}, &types.Artifact{})
	require.NoError(t, err)

	commonDB := &common.Database{DB: db}

	// Setup storage through factory
	tempDir := t.TempDir()
	storageConfig := &config.StorageConfig{
		Type:      "local",
		LocalPath: tempDir,
	}

	factory := storage.NewStorageFactory(storageConfig)
	storageInstance, err := factory.CreateStorage()
	require.NoError(t, err)

	// Create registry service with factory-created storage
	registryService := registry.NewService(commonDB, storageInstance)

	// Create proper npm package
	packageJSON := `{"name": "factory-test-package", "version": "1.0.0"}`
	tarballData, err := createTestNpmTarball(packageJSON)
	require.NoError(t, err)

	// Test basic operation
	ctx := context.Background()
	artifact, err := registryService.Upload(
		ctx,
		"npm",
		"factory-test-package",
		"1.0.0",
		bytes.NewReader(tarballData),
		uuid.New(),
	)

	require.NoError(t, err)
	require.NotNil(t, artifact)
	assert.Equal(t, "factory-test-package", artifact.Name)

	// Verify file exists
	exists, err := storageInstance.Exists(ctx, artifact.StoragePath)
	require.NoError(t, err)
	assert.True(t, exists)
}
