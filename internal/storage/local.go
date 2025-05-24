package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements BlobStorage for local filesystem
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Ensure the base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	
	return &LocalStorage{
		basePath: basePath,
	}, nil
}

// Store saves content to the local filesystem
func (ls *LocalStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	fullPath := filepath.Join(ls.basePath, path)
	
	// Ensure the directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Copy content to file
	if _, err := io.Copy(file, content); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}
	
	return nil
}

// Retrieve gets content from the local filesystem
func (ls *LocalStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(ls.basePath, path)
	
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	
	return file, nil
}

// Delete removes content from the local filesystem
func (ls *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(ls.basePath, path)
	
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	return nil
}

// Exists checks if content exists in the local filesystem
func (ls *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(ls.basePath, path)
	
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}

// GetSize returns the size of content in the local filesystem
func (ls *LocalStorage) GetSize(ctx context.Context, path string) (int64, error) {
	fullPath := filepath.Join(ls.basePath, path)
	
	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	
	return info.Size(), nil
}

// List returns paths matching the prefix in the local filesystem
func (ls *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	searchPath := filepath.Join(ls.basePath, prefix)
	var paths []string
	
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() {
			relPath, err := filepath.Rel(ls.basePath, path)
			if err != nil {
				return err
			}
			paths = append(paths, relPath)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	
	return paths, nil
}
