package storage

import (
	"fmt"

	"github.com/lgulliver/lodestone/pkg/config"
)

// StorageFactory creates storage instances based on configuration
type StorageFactory struct {
	config *config.StorageConfig
}

// NewStorageFactory creates a new storage factory
func NewStorageFactory(config *config.StorageConfig) *StorageFactory {
	return &StorageFactory{config: config}
}

// CreateStorage creates a storage instance based on the configured type
func (sf *StorageFactory) CreateStorage() (BlobStorage, error) {
	switch sf.config.Type {
	case "local":
		return NewLocalStorage(sf.config.LocalPath)
	case "s3":
		// TODO: Implement S3 storage
		return nil, fmt.Errorf("S3 storage not yet implemented")
	case "gcs":
		// TODO: Implement GCS storage
		return nil, fmt.Errorf("GCS storage not yet implemented")
	case "azure":
		// TODO: Implement Azure storage
		return nil, fmt.Errorf("Azure storage not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", sf.config.Type)
	}
}
