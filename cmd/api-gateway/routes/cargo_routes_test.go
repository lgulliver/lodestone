package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCargoPublishSearchInfoDownloadYank(t *testing.T) {
	h := newHarness(t)
	CargoRoutes(h.api, h.registry, h.auth)

	// Publish.
	body, ct := multipartFile(t, "crate", "mycrate-1.0.0.crate", []byte("crate-bytes"))
	w := h.do(http.MethodPut, "/api/cargo/api/v1/crates/new", body, ct)
	assert.Equal(t, http.StatusOK, w.Code)

	// Search.
	w = h.do(http.MethodGet, "/api/cargo/api/v1/crates?q=mycrate", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mycrate")

	// Info.
	w = h.do(http.MethodGet, "/api/cargo/api/v1/crates/mycrate", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1.0.0")

	// Download.
	w = h.do(http.MethodGet, "/api/cargo/api/v1/crates/mycrate/1.0.0/download", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "crate-bytes", w.Body.String())

	// Yank (delete).
	w = h.do(http.MethodDelete, "/api/cargo/api/v1/crates/mycrate/1.0.0/yank", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCargoInfo_NotFound(t *testing.T) {
	h := newHarness(t)
	CargoRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/cargo/api/v1/crates/ghost", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCargoDownload_NotFound(t *testing.T) {
	h := newHarness(t)
	CargoRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/cargo/api/v1/crates/ghost/9.9.9/download", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCargoPublish_BadFilename(t *testing.T) {
	h := newHarness(t)
	CargoRoutes(h.api, h.registry, h.auth)

	body, ct := multipartFile(t, "crate", "mycrate-1.0.0.zip", []byte("x"))
	w := h.do(http.MethodPut, "/api/cargo/api/v1/crates/new", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body, ct = multipartFile(t, "crate", "noversion.crate", []byte("x"))
	w = h.do(http.MethodPut, "/api/cargo/api/v1/crates/new", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body, ct = multipartFile(t, "wrong", "mycrate-1.0.0.crate", []byte("x"))
	w = h.do(http.MethodPut, "/api/cargo/api/v1/crates/new", body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCargoUnauthorized(t *testing.T) {
	h := newHarness(t)
	CargoRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/cargo/api/v1/crates")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
