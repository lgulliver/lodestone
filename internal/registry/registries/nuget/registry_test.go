package nuget

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/pkg/types"
)

// MockStorage implements storage.BlobStorage for testing
type MockStorage struct {
	data map[string][]byte
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		data: make(map[string][]byte),
	}
}

func (m *MockStorage) Store(ctx context.Context, path string, content interface{}, contentType string) error {
	// For testing, just store the path to verify it was called
	m.data[path] = []byte("stored")
	return nil
}

func (m *MockStorage) Retrieve(ctx context.Context, path string) (interface{}, error) {
	return m.data[path], nil
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	delete(m.data, path)
	return nil
}

func (m *MockStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var files []string
	for path := range m.data {
		if len(prefix) == 0 || len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			files = append(files, path)
		}
	}
	return files, nil
}

func (m *MockStorage) Size(ctx context.Context, path string) (int64, error) {
	if data, exists := m.data[path]; exists {
		return int64(len(data)), nil
	}
	return 0, nil
}

func setupTestRegistry(t *testing.T) (*Registry, *common.Database, *MockStorage) {
	mockStorage := NewMockStorage()
	registry := New(mockStorage, nil)
	return registry, nil, mockStorage
}

func TestNew(t *testing.T) {
	mockStorage := NewMockStorage()
	registry := New(mockStorage, nil)

	assert.NotNil(t, registry)
	assert.Equal(t, mockStorage, registry.storage)
}

func TestUpload_Success(t *testing.T) {
	registry, _, mockStorage := setupTestRegistry(t)
	ctx := context.Background()

	artifact := &types.Artifact{
		Name:        "TestPackage",
		Version:     "1.0.0",
		StoragePath: "nuget/testpackage/1.0.0/testpackage.1.0.0.nupkg",
	}

	content := []byte("fake nupkg content")

	err := registry.Upload(ctx, artifact, content)

	assert.NoError(t, err)
	assert.Equal(t, "application/octet-stream", artifact.ContentType)
	assert.Contains(t, mockStorage.data, artifact.StoragePath)
}

func TestValidate_Success(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "TestPackage",
		Version: "1.0.0",
	}

	content := []byte("fake nupkg content")

	err := registry.Validate(artifact, content)

	assert.NoError(t, err)
}

func TestValidate_EmptyContent(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "TestPackage",
		Version: "1.0.0",
	}

	content := []byte{}

	err := registry.Validate(artifact, content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty package content")
}

func TestValidate_InvalidPackageID(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name      string
		packageID string
	}{
		{"starts with hyphen", "-InvalidPackage"},
		{"starts with dot", ".InvalidPackage"},
		{"starts with underscore", "_InvalidPackage"},
		{"contains spaces", "Invalid Package"},
		{"contains special chars", "Invalid@Package"},
		{"empty name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    tt.packageID,
				Version: "1.0.0",
			}

			content := []byte("fake content")

			err := registry.Validate(artifact, content)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid NuGet package ID format")
		})
	}
}

func TestValidate_ValidPackageIDs(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	validIDs := []string{
		"TestPackage",
		"Test.Package",
		"Test-Package",
		"Test_Package",
		"TestPackage123",
		"Microsoft.AspNetCore.App",
		"Newtonsoft.Json",
		"EntityFramework",
		"A",
		"A1",
		"Package.With.Many.Dots",
		"Package-With-Many-Hyphens",
		"Package_With_Many_Underscores",
	}

	for _, packageID := range validIDs {
		t.Run(packageID, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    packageID,
				Version: "1.0.0",
			}

			content := []byte("fake content")

			err := registry.Validate(artifact, content)

			assert.NoError(t, err)
		})
	}
}

func TestValidate_InvalidVersions(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name    string
		version string
	}{
		{"empty version", ""},
		{"single number", "1"},
		{"two numbers", "1.0"},
		{"non-numeric", "abc"},
		{"missing patch", "1.0."},
		{"negative numbers", "-1.0.0"},
		{"leading v", "v1.0.0"},
		{"spaces", "1.0.0 "},
		{"too many dots", "1.0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    "TestPackage",
				Version: tt.version,
			}

			content := []byte("fake content")

			err := registry.Validate(artifact, content)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid semantic version format")
		})
	}
}

func TestValidate_ValidVersions(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	validVersions := []string{
		"1.0.0",
		"0.0.1",
		"10.20.30",
		"1.0.0-alpha",
		"1.0.0-beta.1",
		"1.0.0-rc.1",
		"1.0.0-alpha.beta",
		"1.0.0+build.123",
		"1.0.0-alpha+build.123",
		"2.0.0-rc.1+build.456",
	}

	for _, version := range validVersions {
		t.Run(version, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    "TestPackage",
				Version: version,
			}

			content := []byte("fake content")

			err := registry.Validate(artifact, content)

			assert.NoError(t, err)
		})
	}
}

func TestGetMetadata_Success(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	content := []byte("fake nupkg content")

	metadata, err := registry.GetMetadata(content)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "nuget", metadata["format"])
	assert.Equal(t, "package", metadata["type"])
	assert.Equal(t, ".NET", metadata["framework"])
}

func TestGenerateStoragePath_RegularPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("TestPackage", "1.0.0")

	assert.Equal(t, "nuget/testpackage/1.0.0/testpackage.1.0.0.nupkg", path)
}

func TestGenerateStoragePath_MixedCase(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("Microsoft.AspNetCore.App", "6.0.0")

	assert.Equal(t, "nuget/microsoft.aspnetcore.app/6.0.0/microsoft.aspnetcore.app.6.0.0.nupkg", path)
}

func TestGenerateStoragePath_PreReleaseVersion(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("TestPackage", "1.0.0-alpha")

	assert.Equal(t, "nuget/testpackage/1.0.0-alpha/testpackage.1.0.0-alpha.nupkg", path)
}

func TestDownload_Deprecated(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact, content, err := registry.Download("TestPackage", "1.0.0")

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

	err := registry.Delete("TestPackage", "1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "use service.Delete instead")
}
