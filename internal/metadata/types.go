package metadata

import (
	"time"

	"github.com/google/uuid"
	"github.com/lodestone/pkg/types"
)

// SearchQuery represents a search request
type SearchQuery struct {
	Query     string   `json:"query"`                    // Search term
	Registry  string   `json:"registry"`                 // Filter by registry type
	Publisher string   `json:"publisher"`                // Filter by publisher username
	Tags      []string `json:"tags"`                     // Filter by tags
	IsPublic  *bool    `json:"is_public"`               // Filter by visibility
	SortBy    string   `json:"sort_by"`                 // Sort field: name, created_at, downloads, updated_at
	SortOrder string   `json:"sort_order"`              // Sort order: asc, desc
	Page      int      `json:"page"`                    // Page number (1-based)
	PerPage   int      `json:"per_page"`                // Items per page
}

// SearchResults represents search response
type SearchResults struct {
	Artifacts  []types.Artifact     `json:"artifacts"`
	Pagination types.PaginationInfo `json:"pagination"`
}

// ArtifactMetadata represents detailed metadata for an artifact
type ArtifactMetadata struct {
	Artifact         types.Artifact        `json:"artifact"`
	DownloadStats    DownloadStats         `json:"download_stats"`
	Versions         []types.Artifact      `json:"versions"`
	Dependencies     []Dependency          `json:"dependencies"`
	SecurityInfo     *SecurityInfo         `json:"security_info,omitempty"`
	QualityMetrics   *QualityMetrics       `json:"quality_metrics,omitempty"`
	RegistrySpecific map[string]interface{} `json:"registry_specific,omitempty"`
}

// DownloadStats represents download statistics
type DownloadStats struct {
	Total          int64           `json:"total"`
	Last30Days     int64           `json:"last_30_days"`
	Last7Days      int64           `json:"last_7_days"`
	Today          int64           `json:"today"`
	RecentActivity []DailyDownloads `json:"recent_activity"`
}

// DailyDownloads represents downloads for a specific day
type DailyDownloads struct {
	Date      time.Time `json:"date"`
	Downloads int64     `json:"downloads"`
}

// Dependency represents a package dependency
type Dependency struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	VersionRange string `json:"version_range,omitempty"`
	Optional     bool   `json:"optional,omitempty"`
	DevOnly      bool   `json:"dev_only,omitempty"`
}

// SecurityInfo represents security-related information
type SecurityInfo struct {
	Vulnerabilities []Vulnerability `json:"vulnerabilities,omitempty"`
	SecurityScore   *float64        `json:"security_score,omitempty"`
	LastScanned     *time.Time      `json:"last_scanned,omitempty"`
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string `json:"id,omitempty"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	CVSS        *float64 `json:"cvss,omitempty"`
	FixedIn     string `json:"fixed_in,omitempty"`
}

// QualityMetrics represents code quality metrics
type QualityMetrics struct {
	TestCoverage          *float64 `json:"test_coverage,omitempty"`
	CodeQualityScore      *float64 `json:"code_quality_score,omitempty"`
	DocumentationCoverage *float64 `json:"documentation_coverage,omitempty"`
	Maintainability       *string  `json:"maintainability,omitempty"`
	TechnicalDebt         *string  `json:"technical_debt,omitempty"`
}

// ArtifactIndex represents a search index entry
type ArtifactIndex struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ArtifactID     uuid.UUID `json:"artifact_id" gorm:"type:uuid;uniqueIndex;not null"`
	Name           string    `json:"name" gorm:"index;not null"`
	Registry       string    `json:"registry" gorm:"index;not null"`
	SearchableText string    `json:"searchable_text" gorm:"type:text"`
	Tags           []string  `json:"tags" gorm:"type:jsonb"`
	Description    string    `json:"description" gorm:"type:text"`
	Author         string    `json:"author" gorm:"index"`
	Keywords       []string  `json:"keywords" gorm:"type:jsonb"`
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName sets the table name for ArtifactIndex
func (ArtifactIndex) TableName() string {
	return "artifact_indices"
}

// DownloadEvent represents a download event for analytics
type DownloadEvent struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ArtifactID uuid.UUID `json:"artifact_id" gorm:"type:uuid;index;not null"`
	UserID     *uuid.UUID `json:"user_id" gorm:"type:uuid;index"`
	IPAddress  string    `json:"ip_address" gorm:"index"`
	UserAgent  string    `json:"user_agent"`
	Registry   string    `json:"registry" gorm:"index"`
	Name       string    `json:"name" gorm:"index"`
	Version    string    `json:"version" gorm:"index"`
	Timestamp  time.Time `json:"timestamp" gorm:"index"`
}

// TableName sets the table name for DownloadEvent
func (DownloadEvent) TableName() string {
	return "download_events"
}

// StatsQuery represents a request for statistics
type StatsQuery struct {
	Registry  string     `json:"registry"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	GroupBy   string     `json:"group_by"` // day, week, month
}

// RegistryStats represents statistics for a registry
type RegistryStats struct {
	Registry       string                 `json:"registry"`
	TotalArtifacts int64                  `json:"total_artifacts"`
	TotalDownloads int64                  `json:"total_downloads"`
	UniqueUsers    int64                  `json:"unique_users"`
	PopularItems   []PopularArtifact      `json:"popular_items"`
	RecentActivity []ActivityPoint        `json:"recent_activity"`
	Breakdown      map[string]interface{} `json:"breakdown"`
}

// PopularArtifact represents a popular artifact with stats
type PopularArtifact struct {
	Name      string `json:"name"`
	Registry  string `json:"registry"`
	Downloads int64  `json:"downloads"`
	Author    string `json:"author"`
}

// ActivityPoint represents activity at a point in time
type ActivityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Downloads int64     `json:"downloads"`
	Uploads   int64     `json:"uploads"`
}

// IndexRequest represents a request to index an artifact
type IndexRequest struct {
	ArtifactID uuid.UUID              `json:"artifact_id"`
	Operation  string                 `json:"operation"` // index, update, delete
	Metadata   map[string]interface{} `json:"metadata"`
}

// SearchSuggestion represents a search suggestion
type SearchSuggestion struct {
	Text      string  `json:"text"`
	Type      string  `json:"type"`      // artifact, author, tag
	Score     float64 `json:"score"`
	Registry  string  `json:"registry"`
	Highlight string  `json:"highlight"`
}

// TrendingQuery represents a request for trending artifacts
type TrendingQuery struct {
	Registry string     `json:"registry"`
	Period   string     `json:"period"`    // day, week, month
	Since    *time.Time `json:"since"`
	Limit    int        `json:"limit"`
}

// TrendingArtifact represents a trending artifact
type TrendingArtifact struct {
	Artifact      types.Artifact `json:"artifact"`
	Downloads     int64          `json:"downloads"`
	GrowthRate    float64        `json:"growth_rate"`
	Rank          int            `json:"rank"`
	PreviousRank  *int           `json:"previous_rank,omitempty"`
}
