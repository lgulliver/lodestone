package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoUploadListLatestInfoModZipDelete(t *testing.T) {
	h := newHarness(t)
	GoRoutes(h.api, h.registry, h.auth)

	// Upload module (raw body PUT).
	w := h.do(http.MethodPut, "/api/go/mymod/@v/v1.0.0", []byte("zip-bytes"), "application/zip")
	assert.Equal(t, http.StatusCreated, w.Code)

	// Version list.
	w = h.do(http.MethodGet, "/api/go/mymod/@v/list", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "v1.0.0")

	// @latest.
	w = h.do(http.MethodGet, "/api/go/mymod/@latest", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "v1.0.0")

	// .info
	w = h.do(http.MethodGet, "/api/go/mymod/@v/v1.0.0.info", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "v1.0.0")

	// .mod
	w = h.do(http.MethodGet, "/api/go/mymod/@v/v1.0.0.mod", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "module mymod")

	// .zip
	w = h.do(http.MethodGet, "/api/go/mymod/@v/v1.0.0.zip", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "zip-bytes", w.Body.String())

	// Delete.
	w = h.do(http.MethodDelete, "/api/go/mymod/@v/v1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGoVersionFile_UnsupportedType(t *testing.T) {
	h := newHarness(t)
	GoRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/go/mymod/@v/v1.0.0.txt", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGoLatest_NotFound(t *testing.T) {
	h := newHarness(t)
	GoRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/go/ghost/@latest", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGoInfo_NotFound(t *testing.T) {
	h := newHarness(t)
	GoRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/go/ghost/@v/v9.9.9.info", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGoUpload_InvalidVersion(t *testing.T) {
	h := newHarness(t)
	GoRoutes(h.api, h.registry, h.auth)

	// Non-semver version fails registry validation -> 500 from handler.
	w := h.do(http.MethodPut, "/api/go/mymod/@v/notsemver", []byte("x"), "application/zip")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGoUnauthorized(t *testing.T) {
	h := newHarness(t)
	GoRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/go/mymod/@v/list")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
