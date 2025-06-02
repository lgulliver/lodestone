package oci

import (
	"sync"
	"time"

	"github.com/lgulliver/lodestone/internal/storage"
)

// UploadSession represents an active blob upload session
type UploadSession struct {
	ID         string
	Repository string
	UserID     string
	StartedAt  time.Time
	LastUpdate time.Time
	Size       int64
	Digest     string
	TempPath   string
	mu         sync.RWMutex
}

// SessionManager manages blob upload sessions
type SessionManager struct {
	sessions map[string]*UploadSession
	storage  storage.BlobStorage
	mu       sync.RWMutex
}
