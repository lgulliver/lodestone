package routes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// npmTarball builds a gzip+tar archive containing package/package.json.
func npmTarball(t *testing.T, name, version string, extra map[string]interface{}) []byte {
	t.Helper()
	pkg := map[string]interface{}{"name": name, "version": version}
	for k, v := range extra {
		pkg[k] = v
	}
	data, err := json.Marshal(pkg)
	require.NoError(t, err)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "package/package.json", Mode: 0o644, Size: int64(len(data)),
	}))
	_, err = tw.Write(data)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// npmPublishBody wraps a tarball in the npm publish JSON envelope.
func npmPublishBody(t *testing.T, name, version string, tarball []byte) []byte {
	t.Helper()
	body := map[string]interface{}{
		"_attachments": map[string]interface{}{
			name + "-" + version + ".tgz": map[string]interface{}{
				"data": base64.StdEncoding.EncodeToString(tarball),
			},
		},
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)
	return b
}

func TestNPMPublishInfoVersionDownloadSearchDelete(t *testing.T) {
	h := newHarness(t)
	NPMRoutes(h.api, h.registry, h.auth)

	tb := npmTarball(t, "mypkg", "1.0.0", map[string]interface{}{
		"description": "a package", "author": "Alice",
	})

	// Publish.
	w := h.do(http.MethodPut, "/api/npm/mypkg", npmPublishBody(t, "mypkg", "1.0.0", tb), "application/json")
	require.Equal(t, http.StatusCreated, w.Code)

	// Package info.
	w = h.do(http.MethodGet, "/api/npm/mypkg", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1.0.0")
	assert.Contains(t, w.Body.String(), "dist-tags")

	// Specific version.
	w = h.do(http.MethodGet, "/api/npm/mypkg/1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mypkg@1.0.0")

	// Tarball download.
	w = h.do(http.MethodGet, "/api/npm/mypkg/-/mypkg-1.0.0.tgz", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tb, w.Body.Bytes())

	// Search.
	w = h.do(http.MethodGet, "/api/npm/-/v1/search?text=mypkg", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mypkg")

	// Unpublish whole package (empty version => delete all versions).
	w = h.do(http.MethodDelete, "/api/npm/mypkg/-rev/1-0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Package is gone.
	w = h.do(http.MethodGet, "/api/npm/mypkg", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNPMScopedPublishInfoDownloadDelete(t *testing.T) {
	h := newHarness(t)
	NPMRoutes(h.api, h.registry, h.auth)

	tb := npmTarball(t, "@myscope/mypkg", "1.0.0", nil)

	w := h.do(http.MethodPut, "/api/npm/@myscope/mypkg",
		npmPublishBody(t, "@myscope/mypkg", "1.0.0", tb), "application/json")
	require.Equal(t, http.StatusCreated, w.Code)

	w = h.do(http.MethodGet, "/api/npm/@myscope/mypkg", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1.0.0")

	w = h.do(http.MethodGet, "/api/npm/@myscope/mypkg/1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodGet, "/api/npm/@myscope/mypkg/-/mypkg-1.0.0.tgz", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tb, w.Body.Bytes())

	w = h.do(http.MethodDelete, "/api/npm/@myscope/mypkg/-rev/1-0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNPMInfoVersion_NotFound(t *testing.T) {
	h := newHarness(t)
	NPMRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/npm/ghost", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = h.do(http.MethodGet, "/api/npm/ghost/9.9.9", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = h.do(http.MethodGet, "/api/npm/ghost/-/ghost-9.9.9.tgz", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNPMPublish_BadInput(t *testing.T) {
	h := newHarness(t)
	NPMRoutes(h.api, h.registry, h.auth)

	// Invalid JSON.
	w := h.do(http.MethodPut, "/api/npm/mypkg", []byte("not-json"), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing _attachments.
	w = h.do(http.MethodPut, "/api/npm/mypkg", []byte(`{}`), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Invalid base64 data.
	bad := []byte(`{"_attachments":{"mypkg-1.0.0.tgz":{"data":"!!!not-base64!!!"}}}`)
	w = h.do(http.MethodPut, "/api/npm/mypkg", bad, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Name mismatch between path and package.json.
	tb := npmTarball(t, "otherpkg", "1.0.0", nil)
	w = h.do(http.MethodPut, "/api/npm/mypkg", npmPublishBody(t, "mypkg", "1.0.0", tb), "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNPMUnauthorized(t *testing.T) {
	h := newHarness(t)
	NPMRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/npm/mypkg")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
