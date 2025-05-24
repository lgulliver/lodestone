package storage

import (
	"context"
	"io"
)

// BlobStorage defines the interface for artifact storage
type BlobStorage interface {
	// Store saves content at the given path
	Store(ctx context.Context, path string, content io.Reader, contentType string) error
	
	// Retrieve gets content from the given path
	Retrieve(ctx context.Context, path string) (io.ReadCloser, error)
	
	// Delete removes content at the given path
	Delete(ctx context.Context, path string) error
	
	// Exists checks if content exists at the given path
	Exists(ctx context.Context, path string) (bool, error)
	
	// GetSize returns the size of content at the given path
	GetSize(ctx context.Context, path string) (int64, error)
	
	// List returns paths matching the prefix
	List(ctx context.Context, prefix string) ([]string, error)
}
