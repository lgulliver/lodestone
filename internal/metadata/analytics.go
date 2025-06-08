package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/lgulliver/lodestone/pkg/types"
)

// RecordDownload records a download event for analytics
func (s *Service) RecordDownload(ctx context.Context, artifactID uuid.UUID, userID *uuid.UUID, ipAddress, userAgent string) error {
	log.Debug().
		Str("artifact_id", artifactID.String()).
		Str("ip_address", ipAddress).
		Str("user_agent", userAgent).
		Msg("Recording download event")

	// Get artifact info
	var artifact types.Artifact
	if err := s.db.WithContext(ctx).First(&artifact, artifactID).Error; err != nil {
		log.Error().
			Err(err).
			Str("artifact_id", artifactID.String()).
			Msg("Failed to get artifact for download recording")
		return fmt.Errorf("failed to get artifact: %w", err)
	}

	// Record the download event
	event := &DownloadEvent{
		ArtifactID: artifactID,
		UserID:     userID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Registry:   artifact.Registry,
		Name:       artifact.Name,
		Version:    artifact.Version,
		Timestamp:  time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("failed to record download event: %w", err)
	}

	// Update download count on artifact
	if err := s.db.WithContext(ctx).
		Model(&artifact).
		UpdateColumn("downloads", gorm.Expr("downloads + ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to update download count: %w", err)
	}

	return nil
}

// GetRegistryStats returns statistics for a registry
func (s *Service) GetRegistryStats(ctx context.Context, query *StatsQuery) (*RegistryStats, error) {
	stats := &RegistryStats{
		Registry: query.Registry,
	}

	// Base query with registry filter
	db := s.db.WithContext(ctx)
	if query.Registry != "" {
		db = db.Where("registry = ?", query.Registry)
	}

	// Total artifacts
	if err := db.Model(&types.Artifact{}).Count(&stats.TotalArtifacts).Error; err != nil {
		return nil, fmt.Errorf("failed to count artifacts: %w", err)
	}

	// Total downloads
	var totalDownloads sql.NullInt64
	if err := db.Model(&types.Artifact{}).
		Select("SUM(downloads)").
		Scan(&totalDownloads).Error; err != nil {
		return nil, fmt.Errorf("failed to sum downloads: %w", err)
	}
	stats.TotalDownloads = totalDownloads.Int64

	// Unique users (from download events)
	downloadDB := s.db.WithContext(ctx).Model(&DownloadEvent{})
	if query.Registry != "" {
		downloadDB = downloadDB.Where("registry = ?", query.Registry)
	}
	if query.StartDate != nil {
		downloadDB = downloadDB.Where("timestamp >= ?", *query.StartDate)
	}
	if query.EndDate != nil {
		downloadDB = downloadDB.Where("timestamp <= ?", *query.EndDate)
	}

	if err := downloadDB.Distinct("user_id").Count(&stats.UniqueUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count unique users: %w", err)
	}

	// Popular items
	popularItems, err := s.getPopularItems(ctx, query.Registry, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular items: %w", err)
	}
	stats.PopularItems = popularItems

	// Recent activity
	activity, err := s.getRecentActivity(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}
	stats.RecentActivity = activity

	// Registry-specific breakdown
	breakdown, err := s.getRegistryBreakdown(ctx, query.Registry)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry breakdown: %w", err)
	}
	stats.Breakdown = breakdown

	return stats, nil
}

// GetTrendingArtifacts returns trending artifacts
func (s *Service) GetTrendingArtifacts(ctx context.Context, query *TrendingQuery) ([]TrendingArtifact, error) {
	if query.Limit == 0 {
		query.Limit = 10
	}

	// Calculate the time period
	now := time.Now()
	var startTime time.Time
	switch query.Period {
	case "day":
		startTime = now.AddDate(0, 0, -1)
	case "week":
		startTime = now.AddDate(0, 0, -7)
	case "month":
		startTime = now.AddDate(0, -1, 0)
	default:
		startTime = now.AddDate(0, 0, -7) // Default to week
	}

	if query.Since != nil {
		startTime = *query.Since
	}

	// Get download events for the period
	var events []struct {
		ArtifactID uuid.UUID
		Downloads  int64
	}

	db := s.db.WithContext(ctx).
		Model(&DownloadEvent{}).
		Select("artifact_id, COUNT(*) as downloads").
		Where("timestamp >= ?", startTime).
		Group("artifact_id").
		Order("downloads DESC").
		Limit(query.Limit * 2) // Get more to filter later

	if query.Registry != "" {
		db = db.Where("registry = ?", query.Registry)
	}

	if err := db.Scan(&events).Error; err != nil {
		return nil, fmt.Errorf("failed to get trending data: %w", err)
	}

	// Get previous period data for growth calculation
	previousStart := startTime.Add(-time.Duration(now.Sub(startTime).Nanoseconds()))
	var previousEvents []struct {
		ArtifactID uuid.UUID
		Downloads  int64
	}

	prevDB := s.db.WithContext(ctx).
		Model(&DownloadEvent{}).
		Select("artifact_id, COUNT(*) as downloads").
		Where("timestamp >= ? AND timestamp < ?", previousStart, startTime).
		Group("artifact_id")

	if query.Registry != "" {
		prevDB = prevDB.Where("registry = ?", query.Registry)
	}

	if err := prevDB.Scan(&previousEvents).Error; err != nil {
		return nil, fmt.Errorf("failed to get previous period data: %w", err)
	}

	// Create lookup map for previous downloads
	prevDownloads := make(map[uuid.UUID]int64)
	for _, event := range previousEvents {
		prevDownloads[event.ArtifactID] = event.Downloads
	}

	// Calculate trending artifacts
	var trending []TrendingArtifact
	for i, event := range events {
		if len(trending) >= query.Limit {
			break
		}

		// Get artifact details
		var artifact types.Artifact
		if err := s.db.WithContext(ctx).
			Preload("Publisher").
			First(&artifact, event.ArtifactID).Error; err != nil {
			continue // Skip if artifact not found
		}

		// Calculate growth rate
		prevDowns := prevDownloads[event.ArtifactID]
		var growthRate float64
		if prevDowns > 0 {
			growthRate = float64(event.Downloads-prevDowns) / float64(prevDowns) * 100
		} else if event.Downloads > 0 {
			growthRate = 100 // 100% growth from 0
		}

		trending = append(trending, TrendingArtifact{
			Artifact:   artifact,
			Downloads:  event.Downloads,
			GrowthRate: growthRate,
			Rank:       i + 1,
		})
	}

	// Sort by growth rate for true trending
	sort.Slice(trending, func(i, j int) bool {
		return trending[i].GrowthRate > trending[j].GrowthRate
	})

	// Update ranks after sorting
	for i := range trending {
		trending[i].Rank = i + 1
	}

	return trending, nil
}

// GetSearchSuggestions returns search suggestions
func (s *Service) GetSearchSuggestions(ctx context.Context, query string, limit int) ([]SearchSuggestion, error) {
	if limit == 0 {
		limit = 10
	}

	var suggestions []SearchSuggestion
	searchTerm := "%" + strings.ToLower(query) + "%"

	// Artifact name suggestions
	var artifacts []struct {
		Name      string
		Registry  string
		Downloads int64
	}

	if err := s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Select("name, registry, downloads").
		Where("LOWER(name) LIKE ? AND is_public = ?", searchTerm, true).
		Order("downloads DESC").
		Limit(limit / 2).
		Scan(&artifacts).Error; err == nil {

		for _, artifact := range artifacts {
			suggestions = append(suggestions, SearchSuggestion{
				Text:     artifact.Name,
				Type:     "artifact",
				Registry: artifact.Registry,
				Score:    float64(artifact.Downloads),
			})
		}
	}

	// Author suggestions
	var authors []struct {
		Username string
		Count    int64
	}

	if err := s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Select("users.username, COUNT(*) as count").
		Joins("JOIN users ON artifacts.published_by = users.id").
		Where("LOWER(users.username) LIKE ? AND artifacts.is_public = ?", searchTerm, true).
		Group("users.username").
		Order("count DESC").
		Limit(limit / 4).
		Scan(&authors).Error; err == nil {

		for _, author := range authors {
			suggestions = append(suggestions, SearchSuggestion{
				Text:  author.Username,
				Type:  "author",
				Score: float64(author.Count),
			})
		}
	}

	// Tag suggestions from index
	var tags []struct {
		Tag   string
		Count int64
	}

	if err := s.db.WithContext(ctx).
		Raw(`
			SELECT tag, COUNT(*) as count
			FROM (
				SELECT DISTINCT jsonb_array_elements_text(tags) as tag
				FROM artifact_indices
				WHERE jsonb_array_length(tags) > 0
			) t
			WHERE LOWER(tag) LIKE ?
			GROUP BY tag
			ORDER BY count DESC
			LIMIT ?
		`, searchTerm, limit/4).
		Scan(&tags).Error; err == nil {

		for _, tag := range tags {
			suggestions = append(suggestions, SearchSuggestion{
				Text:  tag.Tag,
				Type:  "tag",
				Score: float64(tag.Count),
			})
		}
	}

	// Sort suggestions by score
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	// Limit final results
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// Helper methods

func (s *Service) getPopularItems(ctx context.Context, registry string, limit int) ([]PopularArtifact, error) {
	var items []PopularArtifact

	db := s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Select("artifacts.name, artifacts.registry, artifacts.downloads, users.username as author").
		Joins("JOIN users ON artifacts.published_by = users.id").
		Where("artifacts.is_public = ?", true).
		Order("artifacts.downloads DESC").
		Limit(limit)

	if registry != "" {
		db = db.Where("artifacts.registry = ?", registry)
	}

	if err := db.Scan(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (s *Service) getRecentActivity(ctx context.Context, query *StatsQuery) ([]ActivityPoint, error) {
	var activity []ActivityPoint

	// Get the date range
	endDate := time.Now()
	if query.EndDate != nil {
		endDate = *query.EndDate
	}

	startDate := endDate.AddDate(0, 0, -30) // Default to 30 days
	if query.StartDate != nil {
		startDate = *query.StartDate
	}

	// Group by day/week/month based on query
	var groupByFormat string
	switch query.GroupBy {
	case "week":
		groupByFormat = "DATE_TRUNC('week', timestamp)"
	case "month":
		groupByFormat = "DATE_TRUNC('month', timestamp)"
	default:
		groupByFormat = "DATE_TRUNC('day', timestamp)"
	}

	// Get download activity
	var downloadActivity []struct {
		Date      time.Time
		Downloads int64
	}

	downloadDB := s.db.WithContext(ctx).
		Model(&DownloadEvent{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as downloads", groupByFormat)).
		Where("timestamp >= ? AND timestamp <= ?", startDate, endDate).
		Group("date").
		Order("date")

	if query.Registry != "" {
		downloadDB = downloadDB.Where("registry = ?", query.Registry)
	}

	if err := downloadDB.Scan(&downloadActivity).Error; err != nil {
		return nil, err
	}

	// Get upload activity (artifact creation)
	var uploadActivity []struct {
		Date    time.Time
		Uploads int64
	}

	uploadDB := s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as uploads",
			strings.Replace(groupByFormat, "timestamp", "created_at", 1))).
		Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Group("date").
		Order("date")

	if query.Registry != "" {
		uploadDB = uploadDB.Where("registry = ?", query.Registry)
	}

	if err := uploadDB.Scan(&uploadActivity).Error; err != nil {
		return nil, err
	}

	// Merge the data
	downloadMap := make(map[string]int64)
	for _, d := range downloadActivity {
		key := d.Date.Format("2006-01-02")
		downloadMap[key] = d.Downloads
	}

	uploadMap := make(map[string]int64)
	for _, u := range uploadActivity {
		key := u.Date.Format("2006-01-02")
		uploadMap[key] = u.Uploads
	}

	// Create activity points
	current := startDate
	for current.Before(endDate) || current.Equal(endDate) {
		key := current.Format("2006-01-02")
		activity = append(activity, ActivityPoint{
			Timestamp: current,
			Downloads: downloadMap[key],
			Uploads:   uploadMap[key],
		})

		switch query.GroupBy {
		case "week":
			current = current.AddDate(0, 0, 7)
		case "month":
			current = current.AddDate(0, 1, 0)
		default:
			current = current.AddDate(0, 0, 1)
		}
	}

	return activity, nil
}

func (s *Service) getRegistryBreakdown(ctx context.Context, registry string) (map[string]interface{}, error) {
	breakdown := make(map[string]interface{})

	if registry != "" {
		// Registry-specific breakdown
		switch registry {
		case "npm":
			breakdown["scoped_packages"] = s.getScopedPackageCount(ctx)
			breakdown["avg_dependencies"] = s.getAvgDependencies(ctx, registry)
		case "nuget":
			breakdown["framework_targets"] = s.getFrameworkTargets(ctx)
			breakdown["package_types"] = s.getPackageTypes(ctx)
		case "maven":
			breakdown["group_ids"] = s.getTopGroupIds(ctx)
			breakdown["java_versions"] = s.getJavaVersions(ctx)
		}
	} else {
		// Overall breakdown by registry
		var registryBreakdown []struct {
			Registry string
			Count    int64
		}

		if err := s.db.WithContext(ctx).
			Model(&types.Artifact{}).
			Select("registry, COUNT(*) as count").
			Group("registry").
			Order("count DESC").
			Scan(&registryBreakdown).Error; err == nil {

			for _, rb := range registryBreakdown {
				breakdown[rb.Registry] = rb.Count
			}
		}
	}

	return breakdown, nil
}

// Registry-specific helper methods

func (s *Service) getScopedPackageCount(ctx context.Context) int64 {
	var count int64
	s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Where("registry = ? AND name LIKE '@%'", "npm").
		Count(&count)
	return count
}

func (s *Service) getAvgDependencies(ctx context.Context, registry string) float64 {
	var avg sql.NullFloat64
	s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Where("registry = ?", registry).
		Select("AVG(jsonb_array_length(metadata->'dependencies'))").
		Scan(&avg)
	return avg.Float64
}

func (s *Service) getFrameworkTargets(ctx context.Context) map[string]int64 {
	var results []struct {
		Target string
		Count  int64
	}

	s.db.WithContext(ctx).
		Raw(`
			SELECT target, COUNT(*) as count
			FROM (
				SELECT jsonb_array_elements_text(metadata->'targetFrameworks') as target
				FROM artifacts
				WHERE registry = 'nuget' AND metadata ? 'targetFrameworks'
			) t
			GROUP BY target
			ORDER BY count DESC
		`).Scan(&results)

	targets := make(map[string]int64)
	for _, r := range results {
		targets[r.Target] = r.Count
	}
	return targets
}

func (s *Service) getPackageTypes(ctx context.Context) map[string]int64 {
	var results []struct {
		Type  string
		Count int64
	}

	s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Select("metadata->>'packageType' as type, COUNT(*) as count").
		Where("registry = ? AND metadata ? 'packageType'", "nuget").
		Group("type").
		Order("count DESC").
		Scan(&results)

	types := make(map[string]int64)
	for _, r := range results {
		types[r.Type] = r.Count
	}
	return types
}

func (s *Service) getTopGroupIds(ctx context.Context) []string {
	var groupIds []string

	s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Where("registry = ?", "maven").
		Select("metadata->>'groupId'").
		Group("metadata->>'groupId'").
		Order("COUNT(*) DESC").
		Limit(10).
		Pluck("metadata->>'groupId'", &groupIds)

	return groupIds
}

func (s *Service) getJavaVersions(ctx context.Context) map[string]int64 {
	var results []struct {
		Version string
		Count   int64
	}

	s.db.WithContext(ctx).
		Model(&types.Artifact{}).
		Select("metadata->>'javaVersion' as version, COUNT(*) as count").
		Where("registry = ? AND metadata ? 'javaVersion'", "maven").
		Group("version").
		Order("count DESC").
		Scan(&results)

	versions := make(map[string]int64)
	for _, r := range results {
		versions[r.Version] = r.Count
	}
	return versions
}
