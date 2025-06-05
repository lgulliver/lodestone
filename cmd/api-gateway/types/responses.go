package types

import (
	"io"
	"time"

	"github.com/google/uuid"
	pkgtypes "github.com/lgulliver/lodestone/pkg/types"
)

// Common HTTP response types used across all API handlers
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	Code    string `json:"code,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	TotalCount int         `json:"totalCount"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
}

// Cross-registry operation types
type UploadRequest struct {
	Registry    string
	PackageName string
	Version     string
	Content     io.Reader
	ContentType string
	UserID      uuid.UUID
	IsSymbol    bool
	Metadata    map[string]interface{}
}

type DownloadResponse struct {
	Artifact *pkgtypes.Artifact
	Content  io.ReadCloser
	Headers  map[string]string
}

type SearchRequest struct {
	Registry string
	Query    string
	Skip     int
	Take     int
	UserID   *uuid.UUID // Optional for authenticated search
	Tags     []string
	Authors  []string
}

type GenericSearchResponse struct {
	TotalHits int           `json:"totalHits"`
	Data      []interface{} `json:"data"`
}

// Package metadata extraction result
type PackageMetadata struct {
	Name        string
	Version     string
	Description string
	Authors     []string
	Tags        []string
	Size        int64
	Hash        string
	ContentType string
	Homepage    string
	License     string
	Repository  string
	Keywords    []string
	PublishedAt time.Time
}

// Validation types
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
	Metadata *PackageMetadata  `json:"metadata,omitempty"`
}

// File upload types for multipart handling
type FileUpload struct {
	Filename    string
	ContentType string
	Size        int64
	Content     []byte
}

// Health check types
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
	Version   string            `json:"version"`
}

// Common registry operation results
type OperationResult struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}
