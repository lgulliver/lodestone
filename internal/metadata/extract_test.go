package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

func newPureService() *Service {
	// Pure extract* helpers do not touch the DB.
	return &Service{}
}

func TestExtractTags(t *testing.T) {
	s := newPureService()
	assert.Equal(t, []string{"a", "b"}, s.extractTags(map[string]interface{}{"tags": []string{"a", "b"}}))
	assert.Equal(t, []string{"x", "y"}, s.extractTags(map[string]interface{}{"tags": []interface{}{"x", "y", 7}}))
	assert.Equal(t, []string{"p", "q"}, s.extractTags(map[string]interface{}{"tags": "p,q"}))
	assert.Empty(t, s.extractTags(map[string]interface{}{}))
	assert.Empty(t, s.extractTags(map[string]interface{}{"tags": 42}))
}

func TestExtractKeywords(t *testing.T) {
	s := newPureService()
	assert.Equal(t, []string{"a", "b"}, s.extractKeywords(map[string]interface{}{"keywords": []string{"a", "b"}}))
	assert.Equal(t, []string{"x"}, s.extractKeywords(map[string]interface{}{"keywords": []interface{}{"x", 1}}))
	assert.Equal(t, []string{"p", "q"}, s.extractKeywords(map[string]interface{}{"keywords": "p,q"}))
	assert.Empty(t, s.extractKeywords(map[string]interface{}{}))
}

func TestExtractDescriptionAndAuthor(t *testing.T) {
	s := newPureService()
	assert.Equal(t, "hello", s.extractDescription(map[string]interface{}{"description": "hello"}))
	assert.Equal(t, "", s.extractDescription(map[string]interface{}{"description": 5}))
	assert.Equal(t, "jane", s.extractAuthor(map[string]interface{}{"author": "jane"}))
	assert.Equal(t, "", s.extractAuthor(map[string]interface{}{}))
}

func TestExtractSearchableText(t *testing.T) {
	s := newPureService()
	art := &types.Artifact{
		Name: "mypkg",
		Metadata: map[string]interface{}{
			"description": "a great package",
			"tags":        []string{"web", "api"},
			"keywords":    []string{"http"},
		},
	}
	text := s.extractSearchableText(art)
	assert.Contains(t, text, "mypkg")
	assert.Contains(t, text, "a great package")
	assert.Contains(t, text, "web api")
	assert.Contains(t, text, "http")

	// Name only when metadata empty.
	assert.Equal(t, "bare", s.extractSearchableText(&types.Artifact{Name: "bare", Metadata: map[string]interface{}{}}))
}

func TestExtractDependencies(t *testing.T) {
	s := newPureService()
	mapForm := s.extractDependencies(map[string]interface{}{
		"dependencies": map[string]interface{}{"left-pad": "1.0.0", "bad": 5},
	})
	require.Len(t, mapForm, 1)
	assert.Equal(t, "left-pad", mapForm[0].Name)

	listForm := s.extractDependencies(map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{"name": "react", "version": "18.0.0"},
			map[string]interface{}{"version": "no-name"}, // skipped
		},
	})
	require.Len(t, listForm, 1)
	assert.Equal(t, "react", listForm[0].Name)

	assert.Empty(t, s.extractDependencies(map[string]interface{}{}))
}

func TestExtractSecurityInfo(t *testing.T) {
	s := newPureService()
	assert.Nil(t, s.extractSecurityInfo(map[string]interface{}{}))

	info := s.extractSecurityInfo(map[string]interface{}{
		"security": map[string]interface{}{
			"score": 8.5,
			"vulnerabilities": []interface{}{
				map[string]interface{}{"severity": "high", "description": "rce"},
			},
		},
	})
	require.NotNil(t, info)
	require.Len(t, info.Vulnerabilities, 1)
	assert.Equal(t, "high", info.Vulnerabilities[0].Severity)
	require.NotNil(t, info.SecurityScore)
	assert.Equal(t, 8.5, *info.SecurityScore)
}

func TestExtractQualityMetrics(t *testing.T) {
	s := newPureService()
	assert.Nil(t, s.extractQualityMetrics(map[string]interface{}{}))

	m := s.extractQualityMetrics(map[string]interface{}{
		"quality": map[string]interface{}{
			"test_coverage":          90.0,
			"code_quality_score":     7.0,
			"documentation_coverage": 50.0,
		},
	})
	require.NotNil(t, m)
	require.NotNil(t, m.TestCoverage)
	assert.Equal(t, 90.0, *m.TestCoverage)
	assert.Equal(t, 7.0, *m.CodeQualityScore)
	assert.Equal(t, 50.0, *m.DocumentationCoverage)
}

func TestExtractRegistrySpecificInfo(t *testing.T) {
	s := newPureService()
	got := s.extractRegistrySpecificInfo("npm", map[string]interface{}{
		"npm": map[string]interface{}{"dist-tags": "latest"},
	})
	assert.Equal(t, "latest", got["dist-tags"])
	assert.Nil(t, s.extractRegistrySpecificInfo("npm", map[string]interface{}{}))
}
