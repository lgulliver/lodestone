package maven

import (
	"bytes"
	"context"
	"io"
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

func (m *MockStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	// Read content from io.Reader
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	m.data[path] = data
	return nil
}

func (m *MockStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	if data, exists := m.data[path]; exists {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, io.EOF
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	delete(m.data, path)
	return nil
}

func (m *MockStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, exists := m.data[path]
	return exists, nil
}

func (m *MockStorage) GetSize(ctx context.Context, path string) (int64, error) {
	if data, exists := m.data[path]; exists {
		return int64(len(data)), nil
	}
	return 0, io.EOF
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

func TestUpload_JarArtifact(t *testing.T) {
	registry, _, mockStorage := setupTestRegistry(t)
	ctx := context.Background()

	artifact := &types.Artifact{
		Name:        "com.example:test-artifact",
		Version:     "1.0.0",
		StoragePath: "maven/com/example/test-artifact/1.0.0/test-artifact-1.0.0.jar",
	}

	content := []byte("fake jar content")

	err := registry.Upload(ctx, artifact, content)

	assert.NoError(t, err)
	assert.Equal(t, "application/java-archive", artifact.ContentType)
	assert.Contains(t, mockStorage.data, artifact.StoragePath)
}

func TestUpload_PomArtifact(t *testing.T) {
	registry, _, mockStorage := setupTestRegistry(t)
	ctx := context.Background()

	artifact := &types.Artifact{
		Name:        "com.example:test-artifact.pom",
		Version:     "1.0.0",
		StoragePath: "maven/com/example/test-artifact/1.0.0/test-artifact-1.0.0.pom",
	}

	content := []byte("<?xml version=\"1.0\"?><project>fake pom</project>")

	err := registry.Upload(ctx, artifact, content)

	assert.NoError(t, err)
	assert.Equal(t, "application/xml", artifact.ContentType)
	assert.Contains(t, mockStorage.data, artifact.StoragePath)
}

func TestUpload_WarArtifact(t *testing.T) {
	registry, _, mockStorage := setupTestRegistry(t)
	ctx := context.Background()

	artifact := &types.Artifact{
		Name:        "com.example:webapp.war",
		Version:     "1.0.0",
		StoragePath: "maven/com/example/webapp/1.0.0/webapp-1.0.0.war",
	}

	content := []byte("fake war content")

	err := registry.Upload(ctx, artifact, content)

	assert.NoError(t, err)
	assert.Equal(t, "application/java-archive", artifact.ContentType)
	assert.Contains(t, mockStorage.data, artifact.StoragePath)
}

func TestValidate_Success(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "com.example:test-artifact",
		Version: "1.0.0",
	}

	content := []byte("fake jar content")

	err := registry.Validate(artifact, content)

	assert.NoError(t, err)
}

func TestValidate_EmptyContent(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact := &types.Artifact{
		Name:    "com.example:test-artifact",
		Version: "1.0.0",
	}

	content := []byte{}

	err := registry.Validate(artifact, content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty artifact content")
}

func TestValidate_InvalidCoordinates(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name           string
		coordinates    string
		expectedErrMsg string
	}{
		{"no colon", "invalid", "invalid Maven coordinates format"},
		{"too many colons", "com.example:test:artifact:extra", "invalid Maven coordinates format"},
		{"empty group", ":test-artifact", "invalid groupId format"},
		{"empty artifact", "com.example:", "invalid artifactId format"},
		{"invalid group chars", "com.example@:test-artifact", "invalid groupId format"},
		{"invalid artifact chars", "com.example:test@artifact", "invalid artifactId format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    tt.coordinates,
				Version: "1.0.0",
			}

			content := []byte("fake content")

			err := registry.Validate(artifact, content)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}

func TestValidate_ValidCoordinates(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	validCoordinates := []string{
		"com.example:test-artifact",
		"org.apache.commons:commons-lang3",
		"io.github.user:my-library",
		"junit:junit",
		"com.fasterxml.jackson.core:jackson-core",
		"org.springframework:spring-core",
		"com.google.guava:guava",
	}

	for _, coordinates := range validCoordinates {
		t.Run(coordinates, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    coordinates,
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
		{"spaces", "1.0.0 "},
		{"invalid chars", "1.0.0@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    "com.example:test-artifact",
				Version: tt.version,
			}

			content := []byte("fake content")

			err := registry.Validate(artifact, content)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid Maven version format")
		})
	}
}

func TestValidate_ValidVersions(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	validVersions := []string{
		"1.0.0",
		"1.0.0-SNAPSHOT",
		"2.1.4.RELEASE",
		"1.0-alpha-1",
		"1.0-beta",
		"1.0-rc1",
		"1.2.3-alpha-123",
		"20220101",
		"1.0.0.Final",
	}

	for _, version := range validVersions {
		t.Run(version, func(t *testing.T) {
			artifact := &types.Artifact{
				Name:    "com.example:test-artifact",
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

	content := []byte("fake jar content")

	metadata, err := registry.GetMetadata(content)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "maven", metadata["format"])
	assert.Equal(t, "library", metadata["type"])
	assert.Equal(t, "Java", metadata["language"])
}

func TestGenerateStoragePath_RegularArtifact(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("com.example:test-artifact", "1.0.0")

	assert.Equal(t, "maven/com/example/test-artifact/1.0.0/test-artifact-1.0.0.jar", path)
}

func TestGenerateStoragePath_ComplexGroupId(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("org.apache.commons:commons-lang3", "3.12.0")

	assert.Equal(t, "maven/org/apache/commons/commons-lang3/3.12.0/commons-lang3-3.12.0.jar", path)
}

func TestGenerateStoragePath_SnapshotVersion(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	path := registry.GenerateStoragePath("com.example:test-artifact", "1.0.0-SNAPSHOT")

	assert.Equal(t, "maven/com/example/test-artifact/1.0.0-SNAPSHOT/test-artifact-1.0.0-SNAPSHOT.jar", path)
}

func TestDownload_Deprecated(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	artifact, content, err := registry.Download("com.example:test-artifact", "1.0.0")

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

	err := registry.Delete("com.example:test-artifact", "1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "use service.Delete instead")
}
