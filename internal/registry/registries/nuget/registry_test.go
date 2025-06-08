package nuget

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
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

// Symbol package test data
var mockSymbolPackageContent = []byte{
	0x50, 0x4b, 0x03, 0x04, // ZIP header
	// Mock ZIP content containing .pdb files
}

func TestIsSymbolPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name     string
		artifact *types.Artifact
		expected bool
	}{
		{
			name: "Symbol package with metadata",
			artifact: &types.Artifact{
				Name:     "TestPackage",
				Version:  "1.0.0",
				Registry: "nuget",
				Metadata: types.JSONMap{
					"packageType": "SymbolsPackage",
					"isSymbols":   true,
				},
				ContentType: "application/vnd.nuget.symbolpackage",
			},
			expected: true,
		},
		{
			name: "Symbol package with content type",
			artifact: &types.Artifact{
				Name:        "TestPackage",
				Version:     "1.0.0",
				Registry:    "nuget",
				ContentType: "application/vnd.nuget.symbolpackage",
			},
			expected: true,
		},
		{
			name: "Regular package",
			artifact: &types.Artifact{
				Name:        "TestPackage",
				Version:     "1.0.0",
				Registry:    "nuget",
				ContentType: "application/zip",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.IsSymbolPackage(tt.artifact)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSymbolStoragePath(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name     string
		pkgName  string
		version  string
		expected string
	}{
		{
			name:     "Simple package",
			pkgName:  "TestPackage",
			version:  "1.0.0",
			expected: "nuget/symbols/testpackage/1.0.0/testpackage.1.0.0.snupkg",
		},
		{
			name:     "Complex package name",
			pkgName:  "Company.Product.Core",
			version:  "2.1.3-beta1",
			expected: "nuget/symbols/company.product.core/2.1.3-beta1/company.product.core.2.1.3-beta1.snupkg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.GenerateSymbolStoragePath(tt.pkgName, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateSymbolPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{
			name:        "Valid symbol package with PDB",
			content:     createMockSymbolPackage(t, []string{"TestLib.pdb", "TestLib.dll"}),
			expectError: false,
		},
		{
			name:        "Valid symbol package with MDB",
			content:     createMockSymbolPackage(t, []string{"TestLib.dll.mdb", "TestLib.dll"}),
			expectError: false,
		},
		{
			name:        "Invalid - no symbol files",
			content:     createMockSymbolPackage(t, []string{"TestLib.dll", "readme.txt"}),
			expectError: true,
		},
		{
			name:        "Empty package",
			content:     []byte{},
			expectError: true,
		},
		{
			name:        "Invalid ZIP",
			content:     []byte("not a zip file"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.validateSymbolPackage(tt.content, "TestPackage", "1.0.0")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSymbolMetadata(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	symbolFiles := []string{"TestLib.pdb", "TestLib2.pdb", "TestLib.dll"}
	content := createMockSymbolPackage(t, symbolFiles)

	metadata, err := registry.GetSymbolMetadata(content)
	assert.NoError(t, err)
	assert.NotNil(t, metadata)

	// Check expected metadata fields
	assert.Equal(t, "SymbolsPackage", metadata["packageType"])
	assert.Equal(t, true, metadata["isSymbols"])
	assert.Equal(t, "application/vnd.nuget.symbolpackage", metadata["contentType"])

	// Check symbol files inventory
	symbolFilesInterface, exists := metadata["symbolFiles"]
	assert.True(t, exists)
	symbolFilesList, ok := symbolFilesInterface.([]string)
	assert.True(t, ok)
	assert.Contains(t, symbolFilesList, "TestLib.pdb")
	assert.Contains(t, symbolFilesList, "TestLib2.pdb")
	assert.NotContains(t, symbolFilesList, "TestLib.dll") // DLL is not a symbol file
}

func TestIsSymbolPackageContent(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "Valid symbol package",
			content:  createMockSymbolPackage(t, []string{"TestLib.pdb", "TestLib.dll"}),
			expected: true,
		},
		{
			name:     "Package without symbols",
			content:  createMockSymbolPackage(t, []string{"TestLib.dll", "readme.txt"}),
			expected: false,
		},
		{
			name:     "Empty content",
			content:  []byte{},
			expected: false,
		},
		{
			name:     "Invalid ZIP",
			content:  []byte("not a zip"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.isSymbolPackageContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateWithSymbolPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	// Test with symbol package
	symbolContent := createMockSymbolPackage(t, []string{"TestLib.pdb", "TestLib.dll"})
	symbolArtifact := &types.Artifact{
		Name:        "TestPackage",
		Version:     "1.0.0",
		Registry:    "nuget",
		ContentType: "application/vnd.nuget.symbolpackage",
	}

	err := registry.Validate(symbolArtifact, symbolContent)
	assert.NoError(t, err)

	// Test with regular package
	regularContent := createMockNuGetPackage(t, "TestPackage", "1.0.0")
	regularArtifact := &types.Artifact{
		Name:        "TestPackage",
		Version:     "1.0.0",
		Registry:    "nuget",
		ContentType: "application/zip",
	}

	err = registry.Validate(regularArtifact, regularContent)
	assert.NoError(t, err)
}

func TestGetMetadataWithSymbolPackage(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	// Test with symbol package
	symbolContent := createMockSymbolPackage(t, []string{"TestLib.pdb", "TestLib2.pdb"})
	metadata, err := registry.GetMetadata(symbolContent)
	assert.NoError(t, err)
	assert.NotNil(t, metadata)

	// Verify symbol package metadata
	assert.Equal(t, "SymbolsPackage", metadata["packageType"])
	assert.Equal(t, true, metadata["isSymbols"])
	assert.Equal(t, "application/vnd.nuget.symbolpackage", metadata["contentType"])

	symbolFilesInterface, exists := metadata["symbolFiles"]
	assert.True(t, exists)
	symbolFilesList, ok := symbolFilesInterface.([]string)
	assert.True(t, ok)
	assert.Len(t, symbolFilesList, 2)
	assert.Contains(t, symbolFilesList, "TestLib.pdb")
	assert.Contains(t, symbolFilesList, "TestLib2.pdb")

	// Test with regular package
	regularContent := createMockNuGetPackage(t, "TestPackage", "1.0.0")
	metadata, err = registry.GetMetadata(regularContent)
	assert.NoError(t, err)
	assert.NotNil(t, metadata)

	// Verify regular package metadata
	assert.Equal(t, "TestPackage", metadata["id"])
	assert.Equal(t, "1.0.0", metadata["version"])
	assert.NotEqual(t, "SymbolsPackage", metadata["packageType"])
}

// Helper functions for testing

func createMockSymbolPackage(t *testing.T, files []string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, filename := range files {
		file, err := w.Create(filename)
		assert.NoError(t, err)

		// Write some mock content
		if strings.HasSuffix(filename, ".pdb") || strings.HasSuffix(filename, ".mdb") {
			file.Write([]byte("mock symbol data for " + filename))
		} else {
			file.Write([]byte("mock content for " + filename))
		}
	}

	err := w.Close()
	assert.NoError(t, err)

	return buf.Bytes()
}

func createMockNuGetPackage(t *testing.T, name, version string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Create a mock .nuspec file
	nuspecContent := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2010/07/nuspec.xsd">
  <metadata>
    <id>%s</id>
    <version>%s</version>
    <title>%s</title>
    <authors>Test Author</authors>
    <description>Test package description</description>
  </metadata>
</package>`, name, version, name)

	nuspecFile, err := w.Create(name + ".nuspec")
	assert.NoError(t, err)
	nuspecFile.Write([]byte(nuspecContent))

	// Create some mock content files
	dllFile, err := w.Create("lib/net48/" + name + ".dll")
	assert.NoError(t, err)
	dllFile.Write([]byte("mock dll content"))

	err = w.Close()
	assert.NoError(t, err)

	return buf.Bytes()
}
