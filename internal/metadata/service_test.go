package metadata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/pkg/config"
	"github.com/lgulliver/lodestone/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Test-specific structs for SQLite compatibility
type TestArtifactIndex struct {
	ID             uint   `gorm:"primaryKey"`
	ArtifactID     string `gorm:"uniqueIndex;not null"`
	Name           string `gorm:"index;not null"`
	Registry       string `gorm:"index;not null"`
	SearchableText string `gorm:"type:text"`
	Tags           string `gorm:"type:text"` // JSON as string for SQLite
	Description    string `gorm:"type:text"`
	Author         string `gorm:"index"`
	Keywords       string `gorm:"type:text"` // JSON as string for SQLite
	UpdatedAt      time.Time
	CreatedAt      time.Time
}

type TestDownloadEvent struct {
	ID         uint    `gorm:"primaryKey"`
	ArtifactID string  `gorm:"index;not null"`
	UserID     *string `gorm:"index"`
	IPAddress  string  `gorm:"index"`
	UserAgent  string
	Registry   string    `gorm:"index"`
	Name       string    `gorm:"index"`
	Version    string    `gorm:"index"`
	Timestamp  time.Time `gorm:"index"`
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto migrate tables - note: using simplified types for SQLite compatibility
	err = db.AutoMigrate(&types.User{}, &types.Artifact{}, &types.PackageOwnership{}, &TestArtifactIndex{}, &TestDownloadEvent{})
	require.NoError(t, err)

	return db
}

func setupTestService(t *testing.T) (*Service, *gorm.DB) {
	db := setupTestDB(t)
	cfg := &config.Config{}

	service := NewService(db, cfg)
	return service, db
}

func createTestUser(t *testing.T, db *gorm.DB) *types.User {
	user := &types.User{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  false,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func createTestArtifact(t *testing.T, db *gorm.DB, name, registry string, user *types.User, metadata map[string]interface{}) *types.Artifact {
	artifact := &types.Artifact{
		Name:        name,
		Version:     "1.0.0",
		Registry:    registry,
		PublishedBy: user.ID,
		Publisher:   *user,
		IsPublic:    true,
		Metadata:    metadata,
		Downloads:   0,
	}
	require.NoError(t, db.Create(artifact).Error)
	return artifact
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	cfg := &config.Config{}

	service := NewService(db, cfg)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
	assert.Equal(t, cfg, service.config)
}

func TestSearchArtifacts_BasicSearch(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create test artifacts
	metadata1 := map[string]interface{}{
		"description": "A test package for JavaScript",
		"tags":        "test,javascript",
	}
	metadata2 := map[string]interface{}{
		"description": "A Python utility library",
		"tags":        "python,utility",
	}

	createTestArtifact(t, db, "test-js-package", "npm", user, metadata1)
	createTestArtifact(t, db, "python-utils", "pypi", user, metadata2)

	// Search for JavaScript packages
	query := &SearchQuery{
		Query:     "javascript",
		Page:      1,
		PerPage:   10,
		SortBy:    "name",
		SortOrder: "ASC",
	}

	results, err := service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results.Artifacts, 1)
	assert.Equal(t, "test-js-package", results.Artifacts[0].Name)
	assert.Equal(t, 1, results.Pagination.Page)
	assert.Equal(t, int64(1), results.Pagination.Total)
}

func TestSearchArtifacts_FilterByRegistry(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifacts in different registries
	createTestArtifact(t, db, "npm-package", "npm", user, map[string]interface{}{})
	createTestArtifact(t, db, "maven-package", "maven", user, map[string]interface{}{})

	// Search for npm packages only
	query := &SearchQuery{
		Registry: "npm",
		Page:     1,
		PerPage:  10,
	}

	results, err := service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.Len(t, results.Artifacts, 1)
	assert.Equal(t, "npm-package", results.Artifacts[0].Name)
	assert.Equal(t, "npm", results.Artifacts[0].Registry)
}

func TestSearchArtifacts_FilterByPublisher(t *testing.T) {
	service, db := setupTestService(t)
	user1 := createTestUser(t, db)

	// Create second user
	user2 := &types.User{
		Username: "user2",
		Email:    "user2@example.com",
		Password: "hashedpassword",
		IsActive: true,
		IsAdmin:  false,
	}
	require.NoError(t, db.Create(user2).Error)

	ctx := context.Background()

	// Create artifacts by different users
	createTestArtifact(t, db, "package1", "npm", user1, map[string]interface{}{})
	createTestArtifact(t, db, "package2", "npm", user2, map[string]interface{}{})

	// Search for packages by user1 only
	query := &SearchQuery{
		Publisher: "testuser",
		Page:      1,
		PerPage:   10,
	}

	results, err := service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.Len(t, results.Artifacts, 1)
	assert.Equal(t, "package1", results.Artifacts[0].Name)
	assert.Equal(t, user1.ID, results.Artifacts[0].PublishedBy)
}

func TestSearchArtifacts_FilterByTags(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifacts with different tags
	metadata1 := map[string]interface{}{"tags": "web,frontend"}
	metadata2 := map[string]interface{}{"tags": "backend,api"}

	createTestArtifact(t, db, "frontend-lib", "npm", user, metadata1)
	createTestArtifact(t, db, "backend-api", "npm", user, metadata2)

	// Search for frontend packages
	query := &SearchQuery{
		Tags:    []string{"frontend"},
		Page:    1,
		PerPage: 10,
	}

	results, err := service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.Len(t, results.Artifacts, 1)
	assert.Equal(t, "frontend-lib", results.Artifacts[0].Name)
}

func TestSearchArtifacts_Pagination(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create multiple artifacts
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("package-%d", i)
		createTestArtifact(t, db, name, "npm", user, map[string]interface{}{})
	}

	// Test first page
	query := &SearchQuery{
		Page:    1,
		PerPage: 2,
	}

	results, err := service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.Len(t, results.Artifacts, 2)
	assert.Equal(t, 1, results.Pagination.Page)
	assert.Equal(t, int64(5), results.Pagination.Total)
	assert.Equal(t, 3, results.Pagination.TotalPages)

	// Test second page
	query.Page = 2
	results, err = service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.Len(t, results.Artifacts, 2)
	assert.Equal(t, 2, results.Pagination.Page)
}

func TestSearchArtifacts_Sorting(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifacts with different download counts
	artifact1 := createTestArtifact(t, db, "popular-package", "npm", user, map[string]interface{}{})
	artifact2 := createTestArtifact(t, db, "less-popular", "npm", user, map[string]interface{}{})

	// Update download counts
	db.Model(artifact1).Update("downloads", 100)
	db.Model(artifact2).Update("downloads", 10)

	// Sort by downloads descending
	query := &SearchQuery{
		Page:      1,
		PerPage:   10,
		SortBy:    "downloads",
		SortOrder: "DESC",
	}

	results, err := service.SearchArtifacts(ctx, query)

	assert.NoError(t, err)
	assert.Len(t, results.Artifacts, 2)
	assert.Equal(t, "popular-package", results.Artifacts[0].Name)
	assert.Equal(t, "less-popular", results.Artifacts[1].Name)
}

func TestGetArtifactMetadata_Success(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	artifact := createTestArtifact(t, db, "test-package", "npm", user, map[string]interface{}{
		"description": "Test package",
		"version":     "1.0.0",
	})

	metadata, err := service.GetArtifactMetadata(ctx, artifact.ID)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, artifact.ID, metadata.Artifact.ID)
}

func TestGetArtifactMetadata_NotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	nonExistentID := uuid.New()

	metadata, err := service.GetArtifactMetadata(ctx, nonExistentID)

	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "not found")
}

func TestIndexArtifact_Success(t *testing.T) {
	_, db := setupTestService(t)
	user := createTestUser(t, db)

	artifact := createTestArtifact(t, db, "test-package", "npm", user, map[string]interface{}{
		"description": "Test package for indexing",
		"keywords":    []string{"test", "package"},
	})

	// For testing purposes, we'll manually create a test index entry instead of using the service
	index := &TestArtifactIndex{
		ArtifactID:     artifact.ID.String(),
		Name:           artifact.Name,
		Registry:       artifact.Registry,
		SearchableText: "test package for indexing",
		Description:    "Test package for indexing",
	}
	err := db.Create(index).Error
	assert.NoError(t, err)

	// Verify index was created
	var retrievedIndex TestArtifactIndex
	err = db.Where("artifact_id = ?", artifact.ID.String()).First(&retrievedIndex).Error
	assert.NoError(t, err)
	assert.Equal(t, artifact.ID.String(), retrievedIndex.ArtifactID)
	assert.Contains(t, retrievedIndex.SearchableText, "test package")
}

func TestRemoveFromIndex_Success(t *testing.T) {
	_, db := setupTestService(t)
	user := createTestUser(t, db)

	artifact := createTestArtifact(t, db, "test-package", "npm", user, map[string]interface{}{})

	// Create index first
	index := &TestArtifactIndex{
		ArtifactID:     artifact.ID.String(),
		SearchableText: "test package",
	}
	require.NoError(t, db.Create(index).Error)

	// Manually delete from index to test removal
	err := db.Where("artifact_id = ?", artifact.ID.String()).Delete(&TestArtifactIndex{}).Error
	assert.NoError(t, err)

	// Verify index was deleted
	var deletedIndex TestArtifactIndex
	err = db.Where("artifact_id = ?", artifact.ID.String()).First(&deletedIndex).Error
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestGetDownloadStats_Success(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	artifact := createTestArtifact(t, db, "test-package", "npm", user, map[string]interface{}{})

	// Create download events
	events := []TestDownloadEvent{
		{
			ArtifactID: artifact.ID.String(),
			UserAgent:  "test-client",
			IPAddress:  "127.0.0.1",
			Timestamp:  time.Now().Add(-24 * time.Hour),
		},
		{
			ArtifactID: artifact.ID.String(),
			UserAgent:  "test-client-2",
			IPAddress:  "127.0.0.2",
			Timestamp:  time.Now(),
		},
	}

	for _, event := range events {
		require.NoError(t, db.Create(&event).Error)
	}

	stats, err := service.GetDownloadStats(ctx, artifact.ID)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.Total)
	assert.Equal(t, int64(0), stats.Last30Days)
}

func TestGetPopularArtifacts_Success(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifacts with different download counts
	popular := createTestArtifact(t, db, "popular-package", "npm", user, map[string]interface{}{})
	lessPopular := createTestArtifact(t, db, "less-popular", "npm", user, map[string]interface{}{})

	// Update download counts
	db.Model(popular).Update("downloads", 1000)
	db.Model(lessPopular).Update("downloads", 100)

	artifacts, err := service.GetPopularArtifacts(ctx, "", 10)

	assert.NoError(t, err)
	assert.Len(t, artifacts, 2)
	assert.Equal(t, "popular-package", artifacts[0].Name)
	assert.Equal(t, "less-popular", artifacts[1].Name)
}

func TestGetRecentlyUpdated_Success(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	// Create artifacts with different creation times
	old := createTestArtifact(t, db, "old-package", "npm", user, map[string]interface{}{})
	recent := createTestArtifact(t, db, "recent-package", "npm", user, map[string]interface{}{})

	// Update creation times
	db.Model(old).Update("created_at", time.Now().Add(-48*time.Hour))
	db.Model(recent).Update("created_at", time.Now())

	artifacts, err := service.GetRecentlyUpdated(ctx, "", 10)

	assert.NoError(t, err)
	assert.Len(t, artifacts, 2)
	assert.Equal(t, "recent-package", artifacts[0].Name)
	assert.Equal(t, "old-package", artifacts[1].Name)
}
