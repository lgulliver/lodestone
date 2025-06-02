package registry

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Registry operation interfaces and types
type RegistryHandler interface {
	Upload(ctx context.Context, artifact *types.Artifact, content io.Reader) error
	Download(ctx context.Context, name, version string) (*types.Artifact, io.ReadCloser, error)
	List(ctx context.Context, filter *types.ArtifactFilter) ([]*types.Artifact, error)
	Delete(ctx context.Context, name, version string) error
	Validate(ctx context.Context, artifact *types.Artifact, content io.Reader) error
	GetMetadata(ctx context.Context, content io.Reader) (map[string]interface{}, error)
	GenerateStoragePath(name, version string) string
}

// Registry-specific validation result
type ValidationResult struct {
	Valid     bool
	Errors    []string
	Warnings  []string
	Metadata  map[string]interface{}
	Size      int64
	FileCount int
}

// Package metadata extraction interface
type MetadataExtractor interface {
	ExtractMetadata(ctx context.Context, content io.Reader) (*PackageMetadata, error)
	ValidateMetadata(ctx context.Context, metadata *PackageMetadata) error
}

// Package metadata structure for registry operations
type PackageMetadata struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Authors      []string               `json:"authors"`
	Tags         []string               `json:"tags"`
	Dependencies map[string]string      `json:"dependencies,omitempty"`
	DevDeps      map[string]string      `json:"devDependencies,omitempty"`
	PeerDeps     map[string]string      `json:"peerDependencies,omitempty"`
	Homepage     string                 `json:"homepage,omitempty"`
	Repository   string                 `json:"repository,omitempty"`
	License      string                 `json:"license,omitempty"`
	Keywords     []string               `json:"keywords,omitempty"`
	Readme       string                 `json:"readme,omitempty"`
	Changelog    string                 `json:"changelog,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// Upload operation context
type UploadContext struct {
	Registry    string
	PackageName string
	Version     string
	UserID      uuid.UUID
	ContentType string
	Size        int64
	IsUpdate    bool
	IsSymbol    bool
	Metadata    map[string]interface{}
}

// Download operation context
type DownloadContext struct {
	Registry    string
	PackageName string
	Version     string
	UserID      *uuid.UUID // Optional for public packages
	IsSymbol    bool
	RequestID   string
}

// Search operation context
type SearchContext struct {
	Registry string
	Query    string
	Filters  map[string]interface{}
	UserID   *uuid.UUID
	Limit    int
	Offset   int
}

// Registry configuration for different package types
type RegistryConfig struct {
	Name                string            `json:"name"`
	Type                string            `json:"type"`
	Enabled             bool              `json:"enabled"`
	Public              bool              `json:"public"`
	AllowAnonymousRead  bool              `json:"allowAnonymousRead"`
	AllowAnonymousWrite bool              `json:"allowAnonymousWrite"`
	MaxPackageSize      int64             `json:"maxPackageSize"`
	AllowedExtensions   []string          `json:"allowedExtensions"`
	Settings            map[string]string `json:"settings"`
}

// Operation audit log entry
type OperationLog struct {
	ID          uuid.UUID              `json:"id"`
	Registry    string                 `json:"registry"`
	Operation   string                 `json:"operation"` // upload, download, delete, search
	PackageName string                 `json:"packageName"`
	Version     string                 `json:"version,omitempty"`
	UserID      uuid.UUID              `json:"userId"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Duration    int64                  `json:"duration"` // milliseconds
	Timestamp   int64                  `json:"timestamp"`
}

// Storage path generation interface
type PathGenerator interface {
	GenerateStoragePath(registry, name, version string) string
	GenerateSymbolPath(registry, name, version string) string
	GenerateMetadataPath(registry, name, version string) string
}

// Package ownership and permissions
type PackagePermission struct {
	PackageName string    `json:"packageName"`
	Registry    string    `json:"registry"`
	UserID      uuid.UUID `json:"userId"`
	Permission  string    `json:"permission"` // owner, maintainer, reader
	GrantedBy   uuid.UUID `json:"grantedBy"`
	GrantedAt   int64     `json:"grantedAt"`
}

// Registry statistics
type RegistryStats struct {
	Registry       string `json:"registry"`
	PackageCount   int64  `json:"packageCount"`
	VersionCount   int64  `json:"versionCount"`
	TotalSize      int64  `json:"totalSize"`
	DownloadCount  int64  `json:"downloadCount"`
	UploadCount    int64  `json:"uploadCount"`
	UniqueUsers    int64  `json:"uniqueUsers"`
	LastActivity   int64  `json:"lastActivity"`
	PopularPackage string `json:"popularPackage,omitempty"`
}
