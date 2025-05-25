package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// LocalStorage implements BlobStorage for local filesystem with production-ready features
type LocalStorage struct {
	basePath string
	mutex    sync.RWMutex // For concurrent access safety
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Ensure the base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		log.Error().Err(err).Str("path", basePath).Msg("failed to create storage directory")
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	log.Info().Str("path", basePath).Msg("local storage initialized")
	return &LocalStorage{
		basePath: basePath,
	}, nil
}

// Store saves content to the local filesystem with atomic writes and integrity checks
func (ls *LocalStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	startTime := time.Now()

	// Check if context is cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	fullPath := filepath.Join(ls.basePath, path)

	// Ensure the directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Error().Err(err).Str("path", path).Str("dir", dir).Msg("failed to create directory")
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file for atomic write
	tempPath := fullPath + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	tempFile, err := os.Create(tempPath)
	if err != nil {
		log.Error().Err(err).Str("path", path).Str("temp_path", tempPath).Msg("failed to create temporary file")
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Ensure cleanup of temp file on failure
	defer func() {
		tempFile.Close()
		if _, err := os.Stat(tempPath); err == nil {
			os.Remove(tempPath)
		}
	}()

	// Create hash writer for integrity verification
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	// Copy content to temp file while calculating checksum
	bytesWritten, err := io.Copy(multiWriter, content)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("failed to write content to temporary file")
		return fmt.Errorf("failed to write content: %w", err)
	}

	// Ensure data is flushed to disk
	if err := tempFile.Sync(); err != nil {
		log.Error().Err(err).Str("path", path).Msg("failed to sync temporary file")
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	tempFile.Close()

	// Atomic move from temp to final location
	if err := os.Rename(tempPath, fullPath); err != nil {
		log.Error().Err(err).Str("path", path).Str("temp_path", tempPath).Msg("failed to move temporary file to final location")
		return fmt.Errorf("failed to move file to final location: %w", err)
	}

	// Calculate checksum for logging
	checksum := hex.EncodeToString(hasher.Sum(nil))
	duration := time.Since(startTime)

	log.Info().
		Str("path", path).
		Str("content_type", contentType).
		Int64("bytes_written", bytesWritten).
		Str("checksum", checksum).
		Dur("duration", duration).
		Msg("file stored successfully")

	return nil
}

// Retrieve gets content from the local filesystem with concurrent access safety
func (ls *LocalStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	startTime := time.Now()
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	fullPath := filepath.Join(ls.basePath, path)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", path).Msg("file not found")
			return nil, fmt.Errorf("file not found: %s", path)
		}
		log.Error().Err(err).Str("path", path).Msg("failed to open file")
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info for logging
	info, _ := file.Stat()
	var size int64
	if info != nil {
		size = info.Size()
	}

	duration := time.Since(startTime)
	log.Debug().
		Str("path", path).
		Int64("size", size).
		Dur("duration", duration).
		Msg("file retrieved successfully")

	return file, nil
}

// Delete removes content from the local filesystem with concurrent access safety
func (ls *LocalStorage) Delete(ctx context.Context, path string) error {
	startTime := time.Now()
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	fullPath := filepath.Join(ls.basePath, path)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if file exists before deletion for better logging
	exists := true
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		exists = false
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", path).Msg("file already deleted or does not exist")
			return nil // Already deleted
		}
		log.Error().Err(err).Str("path", path).Msg("failed to delete file")
		return fmt.Errorf("failed to delete file: %w", err)
	}

	duration := time.Since(startTime)
	if exists {
		log.Info().
			Str("path", path).
			Dur("duration", duration).
			Msg("file deleted successfully")
	}

	return nil
}

// Exists checks if content exists in the local filesystem with concurrent access safety
func (ls *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	fullPath := filepath.Join(ls.basePath, path)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		log.Error().Err(err).Str("path", path).Msg("failed to check file existence")
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetSize returns the size of content in the local filesystem with concurrent access safety
func (ls *LocalStorage) GetSize(ctx context.Context, path string) (int64, error) {
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	fullPath := filepath.Join(ls.basePath, path)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", path).Msg("file not found when getting size")
			return 0, fmt.Errorf("file not found: %s", path)
		}
		log.Error().Err(err).Str("path", path).Msg("failed to get file info")
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	size := info.Size()
	log.Debug().Str("path", path).Int64("size", size).Msg("file size retrieved")

	return size, nil
}

// List returns paths matching the prefix in the local filesystem with concurrent access safety
func (ls *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	startTime := time.Now()
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	searchPath := filepath.Join(ls.basePath, prefix)
	var paths []string

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation during walk
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// Skip directories that don't exist or are inaccessible
			if os.IsNotExist(err) || os.IsPermission(err) {
				log.Debug().Err(err).Str("path", path).Msg("skipping inaccessible path")
				return filepath.SkipDir
			}
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(ls.basePath, path)
			if err != nil {
				log.Error().Err(err).Str("path", path).Msg("failed to get relative path")
				return err
			}
			paths = append(paths, relPath)
		}

		return nil
	})

	if err != nil {
		log.Error().Err(err).Str("prefix", prefix).Msg("failed to list files")
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	duration := time.Since(startTime)
	log.Debug().
		Str("prefix", prefix).
		Int("count", len(paths)).
		Dur("duration", duration).
		Msg("files listed successfully")

	return paths, nil
}
