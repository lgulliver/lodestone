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

func TestStorageFactory_CloudStorageConfigurationValidation(t *testing.T) {
	t.Run("gcs remains unimplemented", func(t *testing.T) {
		storageConfig := &config.StorageConfig{Type: "gcs"}

		factory := NewStorageFactory(storageConfig)
		storage, err := factory.CreateStorage()

		assert.Error(t, err)
		assert.Nil(t, storage)
		assert.Contains(t, err.Error(), "not yet implemented")
	})

	t.Run("s3 requires minimal required config", func(t *testing.T) {
		storageConfig := &config.StorageConfig{
			Type: "s3",
			S3: config.S3StorageConfig{
				Region: "us-east-1",
			},
		}

		factory := NewStorageFactory(storageConfig)
		storage, err := factory.CreateStorage()

		assert.Error(t, err)
		assert.Nil(t, storage)
		assert.Contains(t, err.Error(), "missing required s3 configuration")
	})

	t.Run("azure requires credentials and container", func(t *testing.T) {
		storageConfig := &config.StorageConfig{
			Type: "azure",
			Azure: config.AzureStorageConfig{
				Container: "lodestone-artifacts",
			},
		}

		factory := NewStorageFactory(storageConfig)
		storage, err := factory.CreateStorage()

		assert.Error(t, err)
		assert.Nil(t, storage)
		assert.Contains(t, err.Error(), "missing required azure configuration")
	})
}
