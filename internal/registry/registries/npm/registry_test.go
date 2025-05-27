package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockBlobStorage implements storage.BlobStorage for testing
type MockBlobStorage struct {
	mock.Mock
}

func (m *MockBlobStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	args := m.Called(ctx, path, content, contentType)
	return args.Error(0)
}

func (m *MockBlobStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockBlobStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockBlobStorage) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockBlobStorage) GetSize(ctx context.Context, path string) (int64, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockBlobStorage) List(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	return args.Get(0).([]string), args.Error(1)
}

func setupTestDB(t *testing.T) *common.Database {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto migrate tables
	err = db.AutoMigrate(&types.User{}, &types.Artifact{})
	require.NoError(t, err)

	return &common.Database{DB: db}
}

func setupTestRegistry(t *testing.T) (*Registry, *MockBlobStorage, *common.Database) {
	mockStorage := &MockBlobStorage{}
	db := setupTestDB(t)

	registry := New(mockStorage, db)
	return registry, mockStorage, db
}

// createTestPackageTarball creates a sample npm package tarball for testing
func createTestPackageTarball(packageData map[string]interface{}) ([]byte, error) {
	// Create a buffer to write our tarball to
	var buf bytes.Buffer

	// Create gzip writer
	gw := gzip.NewWriter(&buf)

	// Create tar writer
	tw := tar.NewWriter(gw)

	// Marshal package.json data
	packageJSON, err := json.Marshal(packageData)
	if err != nil {
		return nil, err
	}

	// Add package.json file to tarball
	header := &tar.Header{
		Name:    "package/package.json",
		Mode:    0644,
		Size:    int64(len(packageJSON)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}

	if _, err := tw.Write(packageJSON); err != nil {
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

func TestNew(t *testing.T) {
	mockStorage := &MockBlobStorage{}
	db := setupTestDB(t)

	registry := New(mockStorage, db)

	assert.NotNil(t, registry)
	assert.Equal(t, mockStorage, registry.storage)
	assert.Equal(t, db, registry.db)
}

func TestUpload_Success(t *testing.T) {
	registry, mockStorage, _ := setupTestRegistry(t)
	ctx := context.Background()

	artifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "npm",
		StoragePath: "npm/test-package/1.0.0.tgz",
	}

	content := []byte(`{"name":"test-package","version":"1.0.0"}`)

	// Set up mock expectations
	mockStorage.On("Store", ctx, "npm/test-package/1.0.0.tgz", mock.Anything, "application/octet-stream").Return(nil)

	err := registry.Upload(ctx, artifact, content)

	assert.NoError(t, err)
	assert.Equal(t, "application/octet-stream", artifact.ContentType)
	mockStorage.AssertExpectations(t)
}

func TestUpload_StorageError(t *testing.T) {
	registry, mockStorage, _ := setupTestRegistry(t)
	ctx := context.Background()

	artifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "npm",
		StoragePath: "npm/test-package/1.0.0.tgz",
	}

	content := []byte(`{"name":"test-package","version":"1.0.0"}`)

	// Set up mock expectations - storage fails
	mockStorage.On("Store", ctx, "npm/test-package/1.0.0.tgz", mock.Anything, "application/octet-stream").Return(assert.AnError)

	err := registry.Upload(ctx, artifact, content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store npm package")
	mockStorage.AssertExpectations(t)
}

func TestValidate_Success(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "test-package",
		Version: "1.0.0",
	}

	packageData := map[string]interface{}{
		"name":        "test-package",
		"version":     "1.0.0",
		"description": "A test package",
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	err = registry.Validate(artifact, content)
	assert.NoError(t, err)
}

func TestValidate_ScopedPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "@scope/test-package",
		Version: "1.0.0",
	}

	packageData := map[string]interface{}{
		"name":    "@scope/test-package",
		"version": "1.0.0",
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	err = registry.Validate(artifact, content)
	assert.NoError(t, err)
}

func TestValidate_EmptyContent(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "test-package",
		Version: "1.0.0",
	}

	err := registry.Validate(artifact, []byte{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty package content")
}

func TestValidate_InvalidPackageName(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	testCases := []struct {
		name        string
		packageName string
	}{
		{"uppercase letters", "Test-Package"},
		{"spaces", "test package"},
		{"special characters", "test@package"},
		{"leading dot", ".test-package"},
		{"leading underscore", "_test-package"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    tc.packageName,
				Version: "1.0.0",
			}

			packageData := map[string]interface{}{
				"name":    tc.packageName,
				"version": "1.0.0",
			}

			content, err := createTestPackageTarball(packageData)
			require.NoError(t, err)

			err = registry.Validate(artifact, content)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid npm package name format")
		})
	}
}

func TestValidate_NameMismatch(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "test-package",
		Version: "1.0.0",
	}

	// Content has different package name
	packageData := map[string]interface{}{
		"name":    "different-package",
		"version": "1.0.0",
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	err = registry.Validate(artifact, content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package name mismatch")
}

func TestValidate_VersionMismatch(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "test-package",
		Version: "1.0.0",
	}

	// Content has different version
	packageData := map[string]interface{}{
		"name":    "test-package",
		"version": "2.0.0",
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	err = registry.Validate(artifact, content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package version mismatch")
}

func TestGetMetadata_Success(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	packageData := map[string]interface{}{
		"name":        "test-package",
		"version":     "1.0.0",
		"description": "A test package",
		"license":     "MIT",
		"keywords":    []string{"test", "package"},
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	metadata, err := registry.GetMetadata(content)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "npm", metadata["format"])
	assert.Equal(t, "package", metadata["type"])
	assert.Equal(t, "A test package", metadata["description"])
	assert.Equal(t, "MIT", metadata["license"])

	keywords, ok := metadata["keywords"].([]string)
	assert.True(t, ok)
	assert.Contains(t, keywords, "test")
	assert.Contains(t, keywords, "package")

	// Check time structure
	timeInfo, ok := metadata["time"].(map[string]string)
	assert.True(t, ok)
	assert.Contains(t, timeInfo, "created")
	assert.Contains(t, timeInfo, "modified")
	assert.Contains(t, timeInfo, "1.0.0")
}

func TestGetMetadata_MinimalPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	packageData := map[string]interface{}{
		"name":    "test-package",
		"version": "1.0.0",
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	metadata, err := registry.GetMetadata(content)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "npm", metadata["format"])
	assert.Equal(t, "package", metadata["type"])
	// Should not have description, license, or keywords since they're not in the content
	assert.NotContains(t, metadata, "description")
	assert.NotContains(t, metadata, "license")
	assert.NotContains(t, metadata, "keywords")

	// Check for default dist-tags
	distTags, ok := metadata["dist-tags"].(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "1.0.0", distTags["latest"])
}

func TestGetMetadata_WithDependencies(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	packageData := map[string]interface{}{
		"name":         "test-package",
		"version":      "1.0.0",
		"dependencies": map[string]string{"lodash": "^4.17.21", "express": "^4.18.0"},
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	metadata, err := registry.GetMetadata(content)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)

	dependencies, exists := metadata["dependencies"]
	assert.True(t, exists)

	deps, ok := dependencies.(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "^4.17.21", deps["lodash"])
	assert.Equal(t, "^4.18.0", deps["express"])
}

func TestGenerateStoragePath_RegularPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("test-package", "1.0.0")

	assert.Equal(t, "npm/test-package/1.0.0.tgz", path)
}

func TestGenerateStoragePath_ScopedPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("@scope/test-package", "1.0.0")

	assert.Equal(t, "npm/@scope%2ftest-package/1.0.0.tgz", path)
}

func TestDownload_Deprecated(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact, content, err := registry.Download("test-package", "1.0.0")

	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "use service.Download instead")
}

func TestList_Deprecated(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	filter := &types.ArtifactFilter{}
	artifacts, err := registry.List(filter)

	assert.Error(t, err)
	assert.Nil(t, artifacts)
	assert.Contains(t, err.Error(), "use service.List instead")
}

func TestDelete_Deprecated(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	err := registry.Delete("test-package", "1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "use service.Delete instead")
}
