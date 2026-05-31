package routes

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOPAUploadListDownloadDelete(t *testing.T) {
	h := newHarness(t)
	OPARoutes(h.api, h.registry, h.auth)

	// Upload with explicit version.
	w := h.do(http.MethodPut, "/api/opa/bundles/mybundle/v1.0.0", []byte("rego-bytes"), "application/gzip")
	assert.Equal(t, http.StatusCreated, w.Code)

	// List.
	w = h.do(http.MethodGet, "/api/opa/bundles", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mybundle")

	// Download latest by name.
	w = h.do(http.MethodGet, "/api/opa/bundles/mybundle", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "rego-bytes", w.Body.String())

	// Download specific version.
	w = h.do(http.MethodGet, "/api/opa/bundles/mybundle/v1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete.
	w = h.do(http.MethodDelete, "/api/opa/bundles/mybundle/v1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOPAUpload_DefaultVersionFromHeader(t *testing.T) {
	h := newHarness(t)
	OPARoutes(h.api, h.registry, h.auth)

	// PUT without version path, version supplied via header.
	r := httptest.NewRequest(http.MethodPut, "/api/opa/bundles/headerbundle", bytes.NewReader([]byte("rego")))
	r.Header.Set("Authorization", "Bearer "+h.token)
	r.Header.Set("X-Bundle-Version", "2.0.0")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "v2.0.0")
}

func TestOPADownload_NotFound(t *testing.T) {
	h := newHarness(t)
	OPARoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/opa/bundles/ghost", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = h.do(http.MethodGet, "/api/opa/bundles/ghost/v9.9.9", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOPAUnauthorized(t *testing.T) {
	h := newHarness(t)
	OPARoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/opa/bundles")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
