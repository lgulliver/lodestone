package registry

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

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

// MockHandler implements Handler for testing
type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) Upload(ctx context.Context, artifact *types.Artifact, content []byte) error {
	args := m.Called(ctx, artifact, content)
	return args.Error(0)
}

func (m *MockHandler) Download(name, version string) (*types.Artifact, []byte, error) {
	args := m.Called(name, version)
	return args.Get(0).(*types.Artifact), args.Get(1).([]byte), args.Error(2)
}

func (m *MockHandler) List(filter *types.ArtifactFilter) ([]*types.Artifact, error) {
	args := m.Called(filter)
	return args.Get(0).([]*types.Artifact), args.Error(1)
}

func (m *MockHandler) Delete(name, version string) error {
	args := m.Called(name, version)
	return args.Error(0)
}

func (m *MockHandler) Validate(artifact *types.Artifact, content []byte) error {
	args := m.Called(artifact, content)
	return args.Error(0)
}

func (m *MockHandler) GetMetadata(content []byte) (map[string]interface{}, error) {
	args := m.Called(content)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockHandler) GenerateStoragePath(name, version string) string {
	args := m.Called(name, version)
	return args.String(0)
}

func setupTestDB(t *testing.T) *common.Database {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto migrate tables
	err = db.AutoMigrate(&types.User{}, &types.APIKey{}, &types.Artifact{}, &types.PackageOwnership{})
	require.NoError(t, err)

	return &common.Database{DB: db}
}

func setupTestService(t *testing.T) (*Service, *common.Database, *MockBlobStorage) {
	db := setupTestDB(t)
	mockStorage := &MockBlobStorage{}

	service := NewService(db, mockStorage)
	return service, db, mockStorage
}

func createTestUser(t *testing.T, db *common.Database) *types.User {
	user := &types.User{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  false,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	mockStorage := &MockBlobStorage{}

	service := NewService(db, mockStorage)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.DB)
	assert.Equal(t, mockStorage, service.Storage)
	assert.NotNil(t, service.factory)
	assert.NotEmpty(t, service.handlers)

	// Check that all expected registry types are registered
	expectedTypes := []string{"nuget", "npm", "maven", "go", "helm", "oci", "opa", "cargo", "rubygems"}
	for _, registryType := range expectedTypes {
		_, exists := service.handlers[registryType]
		assert.True(t, exists, "Registry type %s should be registered", registryType)
	}
}

func TestUpload_Success(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create mock handler
	mockHandler := &MockHandler{}
	service.handlers["test"] = mockHandler

	content := []byte("test artifact content")
	reader := bytes.NewReader(content)

	// Set up mock expectations
	mockHandler.On("Validate", mock.AnythingOfType("*types.Artifact"), content).Return(nil)
	mockHandler.On("GetMetadata", content).Return(map[string]interface{}{"test": "metadata"}, nil)
	mockHandler.On("GenerateStoragePath", "test-package", "1.0.0").Return("test/test-package/1.0.0/artifact")
	mockHandler.On("Upload", ctx, mock.AnythingOfType("*types.Artifact"), content).Return(nil)

	// Upload artifact
	artifact, err := service.Upload(ctx, "test", "test-package", "1.0.0", reader, user.ID)

	assert.NoError(t, err)
	assert.NotNil(t, artifact)
	assert.Equal(t, "test-package", artifact.Name)
	assert.Equal(t, "1.0.0", artifact.Version)
	assert.Equal(t, "test", artifact.Registry)
	assert.Equal(t, int64(len(content)), artifact.Size)
	assert.Equal(t, user.ID, artifact.PublishedBy)
	assert.NotEmpty(t, artifact.SHA256)
	assert.Equal(t, "test/test-package/1.0.0/artifact", artifact.StoragePath)

	// Verify artifact was saved to database
	var savedArtifact types.Artifact
	err = db.First(&savedArtifact, artifact.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, artifact.Name, savedArtifact.Name)

	mockHandler.AssertExpectations(t)
}

func TestUpload_UnsupportedRegistry(t *testing.T) {
	service, _, _ := setupTestService(t)
	user := createTestUser(t, service.DB)
	ctx := context.Background()

	content := bytes.NewReader([]byte("test content"))

	artifact, err := service.Upload(ctx, "unsupported", "test-package", "1.0.0", content, user.ID)

	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "unsupported registry type")
}

func TestUpload_ValidationFailed(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create mock handler
	mockHandler := &MockHandler{}
	service.handlers["test"] = mockHandler

	content := []byte("invalid content")
	reader := bytes.NewReader(content)

	// Set up mock expectations - validation fails
	mockHandler.On("Validate", mock.AnythingOfType("*types.Artifact"), content).Return(assert.AnError)

	artifact, err := service.Upload(ctx, "test", "test-package", "1.0.0", reader, user.ID)

	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "validation failed")

	mockHandler.AssertExpectations(t)
}

func TestUpload_DuplicateArtifact(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create existing artifact
	existingArtifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "test",
		PublishedBy: user.ID,
	}
	require.NoError(t, db.Create(existingArtifact).Error)

	// Create mock handler
	mockHandler := &MockHandler{}
	service.handlers["test"] = mockHandler

	content := []byte("test content")
	reader := bytes.NewReader(content)

	// Set up mock expectations
	mockHandler.On("Validate", mock.AnythingOfType("*types.Artifact"), content).Return(nil)
	mockHandler.On("GetMetadata", content).Return(map[string]interface{}{}, nil)
	// Note: GenerateStoragePath should not be called for duplicate artifacts

	artifact, err := service.Upload(ctx, "test", "test-package", "1.0.0", reader, user.ID)

	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "already exists")

	mockHandler.AssertExpectations(t)
}

func TestDownload_Success(t *testing.T) {
	service, db, mockStorage := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifact in database
	artifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "npm",
		StoragePath: "npm/test-package/1.0.0/artifact",
		PublishedBy: user.ID,
		Downloads:   5,
	}
	require.NoError(t, db.Create(artifact).Error)

	// Mock storage response
	contentReader := io.NopCloser(strings.NewReader("test content"))
	mockStorage.On("Retrieve", ctx, "npm/test-package/1.0.0/artifact").Return(contentReader, nil)

	// Download artifact
	downloadedArtifact, content, err := service.Download(ctx, "npm", "test-package", "1.0.0")

	assert.NoError(t, err)
	assert.NotNil(t, downloadedArtifact)
	assert.NotNil(t, content)
	assert.Equal(t, artifact.Name, downloadedArtifact.Name)
	assert.Equal(t, artifact.Version, downloadedArtifact.Version)

	// Verify download counter was incremented
	var updatedArtifact types.Artifact
	db.First(&updatedArtifact, artifact.ID)
	assert.Equal(t, int64(6), updatedArtifact.Downloads)

	mockStorage.AssertExpectations(t)
}

func TestDownload_UnsupportedRegistry(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	artifact, content, err := service.Download(ctx, "unsupported", "test-package", "1.0.0")

	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "unsupported registry type")
}

func TestDownload_ArtifactNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	artifact, content, err := service.Download(ctx, "npm", "nonexistent", "1.0.0")

	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "artifact not found")
}

func TestList_Success(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create test artifacts
	artifacts := []*types.Artifact{
		{
			Name:        "package1",
			Version:     "1.0.0",
			Registry:    "npm",
			PublishedBy: user.ID,
		},
		{
			Name:        "package2",
			Version:     "2.0.0",
			Registry:    "npm",
			PublishedBy: user.ID,
		},
		{
			Name:        "different-package",
			Version:     "1.0.0",
			Registry:    "maven",
			PublishedBy: user.ID,
		},
	}

	for _, artifact := range artifacts {
		require.NoError(t, db.Create(artifact).Error)
	}

	// Test listing all artifacts
	filter := &types.ArtifactFilter{}
	result, total, err := service.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, int64(3), total)

	// Test filtering by registry
	filter = &types.ArtifactFilter{Registry: "npm"}
	result, total, err = service.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(2), total)

	// Test filtering by name
	filter = &types.ArtifactFilter{Name: "package1"}
	result, total, err = service.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "package1", result[0].Name)

	// Test pagination
	filter = &types.ArtifactFilter{Limit: 1, Offset: 1}
	result, total, err = service.List(ctx, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(3), total) // Total should still be 3
}

func TestDelete_Success(t *testing.T) {
	service, db, mockStorage := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifact
	artifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "npm",
		StoragePath: "npm/test-package/1.0.0/artifact",
		PublishedBy: user.ID,
	}
	require.NoError(t, db.Create(artifact).Error)

	// Establish package ownership for the user
	err := service.Ownership.EstablishInitialOwnership(ctx, "npm", "test-package", user.ID)
	require.NoError(t, err)

	// Mock storage deletion
	mockStorage.On("Delete", ctx, "npm/test-package/1.0.0/artifact").Return(nil)

	// Delete artifact
	err = service.Delete(ctx, "npm", "test-package", "1.0.0", user.ID)

	assert.NoError(t, err)

	// Verify artifact was deleted from database
	var deletedArtifact types.Artifact
	err = db.First(&deletedArtifact, artifact.ID).Error
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	mockStorage.AssertExpectations(t)
}

func TestDelete_ArtifactNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	user := createTestUser(t, service.DB)
	ctx := context.Background()

	err := service.Delete(ctx, "npm", "nonexistent", "1.0.0", user.ID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "artifact not found")
}

func TestDelete_InsufficientPermissions(t *testing.T) {
	service, db, _ := setupTestService(t)
	user1 := createTestUser(t, db)

	// Create second user
	user2 := &types.User{
		Username: "user2",
		Email:    "user2@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  false,
	}
	require.NoError(t, db.Create(user2).Error)

	ctx := context.Background()

	// Create artifact owned by user1
	artifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "npm",
		StoragePath: "npm/test-package/1.0.0/artifact",
		PublishedBy: user1.ID,
	}
	require.NoError(t, db.Create(artifact).Error)

	// Try to delete as user2 (should fail)
	err := service.Delete(ctx, "npm", "test-package", "1.0.0", user2.ID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestDelete_AdminCanDelete(t *testing.T) {
	service, db, mockStorage := setupTestService(t)
	user := createTestUser(t, db)

	// Create admin user
	admin := &types.User{
		Username: "admin",
		Email:    "admin@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  true,
	}
	require.NoError(t, db.Create(admin).Error)

	ctx := context.Background()

	// Create artifact owned by regular user
	artifact := &types.Artifact{
		Name:        "test-package",
		Version:     "1.0.0",
		Registry:    "npm",
		StoragePath: "npm/test-package/1.0.0/artifact",
		PublishedBy: user.ID,
	}
	require.NoError(t, db.Create(artifact).Error)

	// Mock storage deletion
	mockStorage.On("Delete", ctx, "npm/test-package/1.0.0/artifact").Return(nil)

	// Admin should be able to delete
	err := service.Delete(ctx, "npm", "test-package", "1.0.0", admin.ID)

	assert.NoError(t, err)

	mockStorage.AssertExpectations(t)
}

func TestGenerateStoragePath(t *testing.T) {
	service, _, _ := setupTestService(t)

	path := service.generateStoragePath("npm", "test-package", "1.0.0")

	assert.Contains(t, path, "npm")
	assert.Contains(t, path, "test-package")
	assert.Contains(t, path, "1.0.0")
	assert.Contains(t, path, ".artifact")
}

func TestGetRegistry_Success(t *testing.T) {
	service, _, _ := setupTestService(t)

	handler, err := service.GetRegistry("npm")

	assert.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestGetRegistry_UnsupportedType(t *testing.T) {
	service, _, _ := setupTestService(t)

	handler, err := service.GetRegistry("unsupported")

	assert.Error(t, err)
	assert.Nil(t, handler)
	assert.Contains(t, err.Error(), "unsupported registry type")
}
