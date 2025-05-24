package registry

import (
	"github.com/lgulliver/lodestone/internal/common"
	"github.com/lgulliver/lodestone/internal/storage"
)

// StorageService provides access to storage and database for registry handlers
type StorageService interface {
	GetStorage() storage.BlobStorage
	GetDB() *common.Database
}

// storageServiceImpl implements StorageService
type storageServiceImpl struct {
	storage storage.BlobStorage
	db      *common.Database
}

// NewStorageService creates a new storage service
func NewStorageService(db *common.Database, storage storage.BlobStorage) StorageService {
	return &storageServiceImpl{
		storage: storage,
		db:      db,
	}
}

// GetStorage returns the storage instance
func (s *storageServiceImpl) GetStorage() storage.BlobStorage {
	return s.storage
}

// GetDB returns the database instance
func (s *storageServiceImpl) GetDB() *common.Database {
	return s.db
}
