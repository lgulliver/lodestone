package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageFactory_CreateLocalStorage(t *testing.T) {
	tempDir := t.TempDir()

	storageConfig := &config.StorageConfig{
		Type:      "local",
		LocalPath: tempDir,
	}

	factory := NewStorageFactory(storageConfig)
	storage, err := factory.CreateStorage()

	require.NoError(t, err)
	require.NotNil(t, storage)

	// Test that we can perform basic operations
	ctx := context.Background()
	testPath := "factory_test.txt"
	testContent := "content from factory test"

	// Store
	err = storage.Store(ctx, testPath, strings.NewReader(testContent), "text/plain")
	assert.NoError(t, err)

	// Verify exists
	exists, err := storage.Exists(ctx, testPath)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Retrieve
	reader, err := storage.Retrieve(ctx, testPath)
	assert.NoError(t, err)
	defer reader.Close()

	// Verify content
	retrievedContent := make([]byte, len(testContent))
	n, err := reader.Read(retrievedContent)
	assert.NoError(t, err)
	assert.Equal(t, len(testContent), n)
	assert.Equal(t, testContent, string(retrievedContent))
}

func TestStorageFactory_UnsupportedType(t *testing.T) {
	storageConfig := &config.StorageConfig{
		Type: "unsupported",
	}

	factory := NewStorageFactory(storageConfig)
	storage, err := factory.CreateStorage()

	assert.Error(t, err)
	assert.Nil(t, storage)
	assert.Contains(t, err.Error(), "unsupported storage type")
}

func TestStorageFactory_CloudStorageNotImplemented(t *testing.T) {
	cloudTypes := []string{"s3", "gcs", "azure"}

	for _, cloudType := range cloudTypes {
		t.Run(cloudType, func(t *testing.T) {
			storageConfig := &config.StorageConfig{
				Type: cloudType,
			}

			factory := NewStorageFactory(storageConfig)
			storage, err := factory.CreateStorage()

			assert.Error(t, err)
			assert.Nil(t, storage)
			assert.Contains(t, err.Error(), "not yet implemented")
		})
	}
}
