package npm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMetadata_RichManifest(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	packageData := map[string]interface{}{
		"name":             "rich-pkg",
		"version":          "2.0.0",
		"description":      "a desc",
		"license":          "MIT",
		"keywords":         []string{"web", "api"},
		"devDependencies":  map[string]string{"jest": "^29.0.0"},
		"peerDependencies": map[string]string{"react": "^18.0.0"},
		"deprecated":       "use v3",
		"homepage":         "http://home",
		"bugs":             map[string]interface{}{"url": "http://bugs"},
		"scripts":          map[string]string{"build": "tsc"},
		"engines":          map[string]string{"node": ">=18"},
		"contributors":     []interface{}{"Alice"},
		"author":           "Bob",
		"repository":       map[string]interface{}{"type": "git", "url": "http://repo"},
		"dist-tags":        map[string]string{"latest": "2.0.0", "beta": "2.1.0-beta"},
		"time":             map[string]interface{}{"2.0.0": "2026-01-01T00:00:00Z"},
	}

	content, err := createTestPackageTarball(packageData)
	require.NoError(t, err)

	meta, err := registry.GetMetadata(content)
	require.NoError(t, err)
	assert.Equal(t, "a desc", meta["description"])
	assert.Equal(t, "MIT", meta["license"])
	assert.Contains(t, meta, "keywords")
	assert.Contains(t, meta, "devDependencies")
	assert.Contains(t, meta, "peerDependencies")
	assert.Equal(t, "use v3", meta["deprecated"])
	assert.Equal(t, "http://home", meta["homepage"])
	assert.Contains(t, meta, "bugs")
	assert.Contains(t, meta, "scripts")
	assert.Contains(t, meta, "engines")
	assert.Contains(t, meta, "contributors")
	assert.Equal(t, "Bob", meta["author"])
	assert.Contains(t, meta, "repository")
	assert.Contains(t, meta, "dist-tags")
	assert.Contains(t, meta, "time")
}

func TestGetMetadata_PrereleaseNoDistTag(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)

	// Prerelease version with no dist-tags: should NOT set a default latest tag.
	content, err := createTestPackageTarball(map[string]interface{}{
		"name":    "pre-pkg",
		"version": "1.0.0-alpha.1",
	})
	require.NoError(t, err)

	meta, err := registry.GetMetadata(content)
	require.NoError(t, err)
	assert.NotContains(t, meta, "dist-tags")
}

func TestGetMetadata_InvalidTarballBasic(t *testing.T) {
	registry, _, _ := setupTestRegistry(t)
	meta, err := registry.GetMetadata([]byte("not a gzip tarball"))
	require.NoError(t, err)
	assert.Equal(t, "npm", meta["format"])
	assert.NotContains(t, meta, "description")
}
