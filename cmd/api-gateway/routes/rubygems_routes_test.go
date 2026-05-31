package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lgulliver/lodestone/pkg/types"
)

func TestGemsSearchInfoVersionsDownload(t *testing.T) {
	h := newHarness(t)
	RubyGemsRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "rubygems", "mygem", "1.0.0", []byte("gem-bytes"),
		types.JSONMap{"description": "a gem", "author": "Alice"})

	w := h.do(http.MethodGet, "/api/gems/api/v1/gems?query=mygem", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mygem")

	w = h.do(http.MethodGet, "/api/gems/gems/mygem-1.0.0.gem", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "gem-bytes", w.Body.String())
}

func TestGemInfoVersions(t *testing.T) {
	h := newHarness(t)
	RubyGemsRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "rubygems", "mygem", "1.0.0", []byte("x"),
		types.JSONMap{"description": "a gem", "author": "Alice"})

	w := h.do(http.MethodGet, "/api/gems/api/v1/gems/mygem.json", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mygem")
	assert.Contains(t, w.Body.String(), "1.0.0")

	w = h.do(http.MethodGet, "/api/gems/api/v1/versions/mygem.json", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1.0.0")

	// Unknown gem → 404.
	w = h.do(http.MethodGet, "/api/gems/api/v1/gems/ghost.json", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGemDownload_BadFilename(t *testing.T) {
	h := newHarness(t)
	RubyGemsRoutes(h.api, h.registry, h.auth)

	// Not a .gem.
	w := h.do(http.MethodGet, "/api/gems/gems/bad.zip", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// .gem but no version separator.
	w = h.do(http.MethodGet, "/api/gems/gems/noversion.gem", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGemYank(t *testing.T) {
	h := newHarness(t)
	RubyGemsRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "rubygems", "yankme", "2.0.0", []byte("x"), nil)

	w := h.do(http.MethodDelete, "/api/gems/api/v1/gems/yank?gem_name=yankme&version=2.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Missing params.
	w = h.do(http.MethodDelete, "/api/gems/api/v1/gems/yank", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGemPush_BadFilename(t *testing.T) {
	h := newHarness(t)
	RubyGemsRoutes(h.api, h.registry, h.auth)

	body, ct := multipartFile(t, "gem", "mygem-1.0.0.zip", []byte("x"))
	w := h.do(http.MethodPost, "/api/gems/api/v1/gems", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body, ct = multipartFile(t, "gem", "noversion.gem", []byte("x"))
	w = h.do(http.MethodPost, "/api/gems/api/v1/gems", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGemSpecsEndpoints(t *testing.T) {
	h := newHarness(t)
	RubyGemsRoutes(h.api, h.registry, h.auth)

	for _, p := range []string{"/api/gems/specs.4.8.gz", "/api/gems/latest_specs.4.8.gz", "/api/gems/prerelease_specs.4.8.gz"} {
		w := h.do(http.MethodGet, p, nil, "")
		assert.Equal(t, http.StatusOK, w.Code)
	}
}
