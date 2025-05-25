package metadata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
)

// Service handles metadata operations including search and indexing
type Service struct {
	db     *gorm.DB
	config *config.Config
}

// NewService creates a new metadata service
func NewService(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{
		db:     db,
		config: cfg,
	}
}

// SearchArtifacts searches artifacts by name, description, tags, etc.
func (s *Service) SearchArtifacts(ctx context.Context, query *SearchQuery) (*SearchResults, error) {
	var artifacts []types.Artifact
	var total int64

	// Build the base query
	db := s.db.WithContext(ctx).Model(&types.Artifact{})

	// Apply filters
	if query.Query != "" {
		// Search in name, description, and metadata
		searchTerm := "%" + strings.ToLower(query.Query) + "%"
		db = db.Where(
			"LOWER(name) LIKE ? OR LOWER(metadata->>'description') LIKE ? OR LOWER(metadata->>'tags') LIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	}

	if query.Registry != "" {
		db = db.Where("registry = ?", query.Registry)
	}

	if query.Publisher != "" {
		db = db.Joins("JOIN users ON artifacts.published_by = users.id").
			Where("users.username = ?", query.Publisher)
	}

	if len(query.Tags) > 0 {
		for _, tag := range query.Tags {
			db = db.Where("metadata->>'tags' LIKE ?", "%"+tag+"%")
		}
	}

	if query.IsPublic != nil {
		db = db.Where("is_public = ?", *query.IsPublic)
	}

	// Count total results
	if err := db.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply sorting
	switch query.SortBy {
	case "name":
		db = db.Order("artifacts.name " + query.SortOrder)
	case "created_at":
		db = db.Order("artifacts.created_at " + query.SortOrder)
	case "downloads":
		db = db.Order("artifacts.downloads " + query.SortOrder)
	case "updated_at":
		db = db.Order("artifacts.updated_at " + query.SortOrder)
	default:
		db = db.Order("artifacts.created_at DESC")
	}

	// Apply pagination
	offset := (query.Page - 1) * query.PerPage
	db = db.Offset(offset).Limit(query.PerPage)

	// Load with associations
	if err := db.Preload("Publisher").Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("failed to search artifacts: %w", err)
	}

	// Calculate pagination info
	totalPages := int((total + int64(query.PerPage) - 1) / int64(query.PerPage))

	return &SearchResults{
		Artifacts: artifacts,
		Pagination: types.PaginationInfo{
			Page:       query.Page,
			PerPage:    query.PerPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetArtifactMetadata retrieves detailed metadata for an artifact
func (s *Service) GetArtifactMetadata(ctx context.Context, artifactID uuid.UUID) (*ArtifactMetadata, error) {
	var artifact types.Artifact
	if err := s.db.WithContext(ctx).
		Preload("Publisher").
		First(&artifact, artifactID).Error; err != nil {
		return nil, fmt.Errorf("failed to get artifact: %w", err)
	}

	// Get download statistics
	stats, err := s.GetDownloadStats(ctx, artifactID)
	if err != nil {
		return nil, fmt.Errorf("failed to get download stats: %w", err)
	}

	// Get related artifacts (same name, different versions)
	var versions []types.Artifact
	if err := s.db.WithContext(ctx).
		Where("name = ? AND registry = ? AND id != ?", artifact.Name, artifact.Registry, artifactID).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("failed to get artifact versions: %w", err)
	}

	// Get dependencies if available in metadata
	dependencies := s.extractDependencies(artifact.Metadata)

	return &ArtifactMetadata{
		Artifact:         artifact,
		DownloadStats:    *stats,
		Versions:         versions,
		Dependencies:     dependencies,
		SecurityInfo:     s.extractSecurityInfo(artifact.Metadata),
		QualityMetrics:   s.extractQualityMetrics(artifact.Metadata),
		RegistrySpecific: s.extractRegistrySpecificInfo(artifact.Registry, artifact.Metadata),
	}, nil
}

// UpdateArtifactMetadata updates metadata for an artifact
func (s *Service) UpdateArtifactMetadata(ctx context.Context, artifactID uuid.UUID, metadata map[string]interface{}) error {
	// Merge with existing metadata
	var artifact types.Artifact
	if err := s.db.WithContext(ctx).First(&artifact, artifactID).Error; err != nil {
		return fmt.Errorf("failed to get artifact: %w", err)
	}

	// Merge metadata
	if artifact.Metadata == nil {
		artifact.Metadata = make(map[string]interface{})
	}

	for key, value := range metadata {
		artifact.Metadata[key] = value
	}

	// Update the artifact
	if err := s.db.WithContext(ctx).
		Model(&artifact).
		Update("metadata", artifact.Metadata).Error; err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Index the updated metadata
	if err := s.IndexArtifact(ctx, &artifact); err != nil {
		return fmt.Errorf("failed to index updated artifact: %w", err)
	}

	return nil
}

// IndexArtifact adds or updates an artifact in the search index
func (s *Service) IndexArtifact(ctx context.Context, artifact *types.Artifact) error {
	// For now, we'll use database indexing
	// In production, you might want to integrate with Elasticsearch or similar

	// Extract searchable text from metadata
	searchableText := s.extractSearchableText(artifact)

	// Store in a search index table (we'll create this)
	index := &ArtifactIndex{
		ArtifactID:      artifact.ID,
		Name:            artifact.Name,
		Registry:        artifact.Registry,
		SearchableText:  searchableText,
		Tags:            s.extractTags(artifact.Metadata),
		Description:     s.extractDescription(artifact.Metadata),
		Author:          s.extractAuthor(artifact.Metadata),
		Keywords:        s.extractKeywords(artifact.Metadata),
		UpdatedAt:       time.Now(),
	}

	// Upsert the index entry
	if err := s.db.WithContext(ctx).
		Where("artifact_id = ?", artifact.ID).
		Assign(index).
		FirstOrCreate(index).Error; err != nil {
		return fmt.Errorf("failed to index artifact: %w", err)
	}

	return nil
}

// RemoveFromIndex removes an artifact from the search index
func (s *Service) RemoveFromIndex(ctx context.Context, artifactID uuid.UUID) error {
	if err := s.db.WithContext(ctx).
		Where("artifact_id = ?", artifactID).
		Delete(&ArtifactIndex{}).Error; err != nil {
		return fmt.Errorf("failed to remove from index: %w", err)
	}
	return nil
}

// GetDownloadStats retrieves download statistics for an artifact
func (s *Service) GetDownloadStats(ctx context.Context, artifactID uuid.UUID) (*DownloadStats, error) {
	var artifact types.Artifact
	if err := s.db.WithContext(ctx).First(&artifact, artifactID).Error; err != nil {
		return nil, fmt.Errorf("failed to get artifact: %w", err)
	}

	// Get recent download activity (you might want to implement a downloads table)
	now := time.Now()

	// For now, we'll simulate this - in production you'd track individual downloads
	stats := &DownloadStats{
		Total:     artifact.Downloads,
		Last30Days: artifact.Downloads / 10, // Simulate recent activity
		Last7Days:  artifact.Downloads / 30,
		Today:      artifact.Downloads / 100,
		RecentActivity: []DailyDownloads{
			{Date: now.AddDate(0, 0, -1), Downloads: 5},
			{Date: now.AddDate(0, 0, -2), Downloads: 3},
			{Date: now.AddDate(0, 0, -3), Downloads: 8},
		},
	}

	return stats, nil
}

// GetPopularArtifacts returns the most popular artifacts
func (s *Service) GetPopularArtifacts(ctx context.Context, registry string, limit int) ([]types.Artifact, error) {
	var artifacts []types.Artifact
	db := s.db.WithContext(ctx).Preload("Publisher")

	if registry != "" {
		db = db.Where("registry = ?", registry)
	}

	if err := db.Where("is_public = ?", true).
		Order("downloads DESC").
		Limit(limit).
		Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("failed to get popular artifacts: %w", err)
	}

	return artifacts, nil
}

// GetRecentlyUpdated returns recently updated artifacts
func (s *Service) GetRecentlyUpdated(ctx context.Context, registry string, limit int) ([]types.Artifact, error) {
	var artifacts []types.Artifact
	db := s.db.WithContext(ctx).Preload("Publisher")

	if registry != "" {
		db = db.Where("registry = ?", registry)
	}

	if err := db.Where("is_public = ?", true).
		Order("updated_at DESC").
		Limit(limit).
		Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("failed to get recently updated artifacts: %w", err)
	}

	return artifacts, nil
}

// Helper methods for extracting metadata

func (s *Service) extractSearchableText(artifact *types.Artifact) string {
	var parts []string
	parts = append(parts, artifact.Name)

	if desc := s.extractDescription(artifact.Metadata); desc != "" {
		parts = append(parts, desc)
	}

	if tags := s.extractTags(artifact.Metadata); len(tags) > 0 {
		parts = append(parts, strings.Join(tags, " "))
	}

	if keywords := s.extractKeywords(artifact.Metadata); len(keywords) > 0 {
		parts = append(parts, strings.Join(keywords, " "))
	}

	return strings.Join(parts, " ")
}

func (s *Service) extractTags(metadata map[string]interface{}) []string {
	if tags, ok := metadata["tags"]; ok {
		switch t := tags.(type) {
		case []string:
			return t
		case []interface{}:
			var result []string
			for _, tag := range t {
				if str, ok := tag.(string); ok {
					result = append(result, str)
				}
			}
			return result
		case string:
			return strings.Split(t, ",")
		}
	}
	return []string{}
}

func (s *Service) extractDescription(metadata map[string]interface{}) string {
	if desc, ok := metadata["description"].(string); ok {
		return desc
	}
	return ""
}

func (s *Service) extractAuthor(metadata map[string]interface{}) string {
	if author, ok := metadata["author"].(string); ok {
		return author
	}
	return ""
}

func (s *Service) extractKeywords(metadata map[string]interface{}) []string {
	if keywords, ok := metadata["keywords"]; ok {
		switch k := keywords.(type) {
		case []string:
			return k
		case []interface{}:
			var result []string
			for _, keyword := range k {
				if str, ok := keyword.(string); ok {
					result = append(result, str)
				}
			}
			return result
		case string:
			return strings.Split(k, ",")
		}
	}
	return []string{}
}

func (s *Service) extractDependencies(metadata map[string]interface{}) []Dependency {
	var deps []Dependency

	if dependencies, ok := metadata["dependencies"]; ok {
		switch d := dependencies.(type) {
		case map[string]interface{}:
			for name, version := range d {
				if vStr, ok := version.(string); ok {
					deps = append(deps, Dependency{
						Name:    name,
						Version: vStr,
					})
				}
			}
		case []interface{}:
			for _, dep := range d {
				if depMap, ok := dep.(map[string]interface{}); ok {
					name, _ := depMap["name"].(string)
					version, _ := depMap["version"].(string)
					if name != "" {
						deps = append(deps, Dependency{
							Name:    name,
							Version: version,
						})
					}
				}
			}
		}
	}

	return deps
}

func (s *Service) extractSecurityInfo(metadata map[string]interface{}) *SecurityInfo {
	if security, ok := metadata["security"].(map[string]interface{}); ok {
		info := &SecurityInfo{}
		
		if vulnerabilities, ok := security["vulnerabilities"].([]interface{}); ok {
			for _, vuln := range vulnerabilities {
				if vulnMap, ok := vuln.(map[string]interface{}); ok {
					severity, _ := vulnMap["severity"].(string)
					description, _ := vulnMap["description"].(string)
					info.Vulnerabilities = append(info.Vulnerabilities, Vulnerability{
						Severity:    severity,
						Description: description,
					})
				}
			}
		}

		if score, ok := security["score"].(float64); ok {
			info.SecurityScore = &score
		}

		return info
	}
	return nil
}

func (s *Service) extractQualityMetrics(metadata map[string]interface{}) *QualityMetrics {
	if quality, ok := metadata["quality"].(map[string]interface{}); ok {
		metrics := &QualityMetrics{}
		
		if coverage, ok := quality["test_coverage"].(float64); ok {
			metrics.TestCoverage = &coverage
		}

		if score, ok := quality["code_quality_score"].(float64); ok {
			metrics.CodeQualityScore = &score
		}

		if docs, ok := quality["documentation_coverage"].(float64); ok {
			metrics.DocumentationCoverage = &docs
		}

		return metrics
	}
	return nil
}

func (s *Service) extractRegistrySpecificInfo(registry string, metadata map[string]interface{}) map[string]interface{} {
	if registryInfo, ok := metadata[registry].(map[string]interface{}); ok {
		return registryInfo
	}
	return nil
}
