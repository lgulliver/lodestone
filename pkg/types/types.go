package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONMap is a custom type that can handle JSON serialization for both PostgreSQL and SQLite
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface for GORM
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for GORM
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSONMap", value)
	}

	return json.Unmarshal(bytes, j)
}

// User represents a user in the system
type User struct {
	ID        uuid.UUID `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"uniqueIndex;not null"`
	Email     string    `json:"email" gorm:"uniqueIndex;not null"`
	Password  string    `json:"-" gorm:"not null"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	IsAdmin   bool      `json:"is_admin" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate generates a UUID for the user ID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID          uuid.UUID  `json:"id" gorm:"primaryKey"`
	UserID      uuid.UUID  `json:"user_id" gorm:"not null"`
	Name        string     `json:"name" gorm:"not null"`
	KeyHash     string     `json:"-" gorm:"not null"`
	Permissions []string   `json:"permissions" gorm:"serializer:json"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	User        User       `json:"user" gorm:"foreignKey:UserID"`
}

// BeforeCreate generates a UUID for the API key ID
func (a *APIKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Artifact represents a stored artifact
type Artifact struct {
	ID          uuid.UUID `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Version     string    `json:"version" gorm:"not null"`
	Registry    string    `json:"registry" gorm:"not null"` // nuget, npm, maven, etc.
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256" gorm:"index"`
	StoragePath string    `json:"-" gorm:"not null"`
	Metadata    JSONMap   `json:"metadata" gorm:"serializer:json"`
	Downloads   int64     `json:"downloads" gorm:"default:0"`
	PublishedBy uuid.UUID `json:"published_by"`
	IsPublic    bool      `json:"is_public" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Publisher   User      `json:"publisher" gorm:"foreignKey:PublishedBy"`
}

// BeforeCreate generates a UUID for the artifact ID
func (a *Artifact) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Permission represents a permission in the system
type Permission struct {
	ID            uuid.UUID `json:"id" gorm:"primaryKey"`
	UserID        uuid.UUID `json:"user_id" gorm:"not null"`
	Resource      string    `json:"resource" gorm:"not null"` // registry:nuget, package:lodestone/myapp
	Action        string    `json:"action" gorm:"not null"`   // read, write, delete
	GrantedBy     uuid.UUID `json:"granted_by"`
	CreatedAt     time.Time `json:"created_at"`
	User          User      `json:"user" gorm:"foreignKey:UserID"`
	GrantedByUser User      `json:"granted_by_user" gorm:"foreignKey:GrantedBy"`
}

// BeforeCreate generates a UUID for the permission ID
func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// PackageOwnership represents ownership of a package
type PackageOwnership struct {
	ID         uuid.UUID `json:"id" gorm:"primaryKey"`
	PackageKey string    `json:"package_key" gorm:"not null;index"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	Role       string    `json:"role" gorm:"not null"`
	GrantedBy  uuid.UUID `json:"granted_by" gorm:"type:uuid;not null"`
	GrantedAt  time.Time `json:"granted_at" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relationships
	User          User `json:"user" gorm:"foreignKey:UserID"`
	GrantedByUser User `json:"granted_by_user" gorm:"foreignKey:GrantedBy"`
}

// Registry interface for different artifact types
type Registry interface {
	// Upload stores an artifact
	Upload(artifact *Artifact, content []byte) error

	// Download retrieves an artifact
	Download(name, version string) (*Artifact, []byte, error)

	// List returns artifacts matching the filter
	List(filter *ArtifactFilter) ([]*Artifact, error)

	// Delete removes an artifact
	Delete(name, version string) error

	// Validate checks if the artifact is valid for this registry
	Validate(artifact *Artifact, content []byte) error

	// GetMetadata extracts metadata from the artifact
	GetMetadata(content []byte) (map[string]interface{}, error)
}

// ArtifactFilter for searching artifacts
type ArtifactFilter struct {
	Name     string   `json:"name"`
	Registry string   `json:"registry"`
	Tags     []string `json:"tags"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
}

// RegistryType represents supported registry types
type RegistryType string

const (
	RegistryNuGet    RegistryType = "nuget"
	RegistryOCI      RegistryType = "oci"
	RegistryOPA      RegistryType = "opa"
	RegistryMaven    RegistryType = "maven"
	RegistryNPM      RegistryType = "npm"
	RegistryCargo    RegistryType = "cargo"
	RegistryGo       RegistryType = "go"
	RegistryHelm     RegistryType = "helm"
	RegistryRubyGems RegistryType = "rubygems"
)

// AuthToken represents a JWT token
type AuthToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	UserID    uuid.UUID `json:"user_id"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	APIResponse
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}
