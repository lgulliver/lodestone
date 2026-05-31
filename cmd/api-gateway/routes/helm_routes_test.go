package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmUploadIndexDownloadDelete(t *testing.T) {
	h := newHarness(t)
	HelmRoutes(h.api, h.registry, h.auth)

	// Upload a chart.
	body, ct := multipartFile(t, "chart", "mychart-1.0.0.tgz", []byte("chart-bytes"))
	w := h.do(http.MethodPost, "/api/helm/api/charts", body, ct)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Index lists it.
	w = h.do(http.MethodGet, "/api/helm/index.yaml", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mychart")

	// Download it.
	w = h.do(http.MethodGet, "/api/helm/mychart/1.0.0/mychart-1.0.0.tgz", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "chart-bytes", w.Body.String())

	// Delete it.
	w = h.do(http.MethodDelete, "/api/helm/api/charts/mychart/1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHelmUpload_BadFilename(t *testing.T) {
	h := newHarness(t)
	HelmRoutes(h.api, h.registry, h.auth)

	// Not a .tgz.
	body, ct := multipartFile(t, "chart", "mychart-1.0.0.zip", []byte("x"))
	w := h.do(http.MethodPost, "/api/helm/api/charts", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing version segment.
	body, ct = multipartFile(t, "chart", "mychart.tgz", []byte("x"))
	w = h.do(http.MethodPost, "/api/helm/api/charts", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Wrong form field name.
	body, ct = multipartFile(t, "wrong", "mychart-1.0.0.tgz", []byte("x"))
	w = h.do(http.MethodPost, "/api/helm/api/charts", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHelmDownload_NotFound(t *testing.T) {
	h := newHarness(t)
	HelmRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/helm/ghost/9.9.9/ghost-9.9.9.tgz", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHelmUnauthorized(t *testing.T) {
	h := newHarness(t)
	HelmRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/helm/index.yaml")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
