package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blobDigest returns the sha256: digest string for the given bytes.
func blobDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// dockerManifest is a minimal but valid v2 manifest JSON. PutManifest only
// requires the body to parse as JSON for the docker/oci content types.
func dockerManifest(t *testing.T) []byte {
	t.Helper()
	m := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
		"config":        map[string]interface{}{"mediaType": "application/vnd.docker.container.image.v1+json", "size": 0, "digest": "sha256:" + hex.EncodeToString(make([]byte, 32))},
		"layers":        []interface{}{},
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

const dockerManifestType = "application/vnd.docker.distribution.manifest.v2+json"

func TestOCIBlobPushPullDelete(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	blob := []byte("hello-layer-bytes")
	digest := blobDigest(blob)

	// Start upload session.
	w := h.do(http.MethodPost, "/api/v2/myrepo/blobs/uploads/", nil, "")
	require.Equal(t, http.StatusAccepted, w.Code)
	uuid := w.Header().Get("Docker-Upload-UUID")
	require.NotEmpty(t, uuid)

	// Status check.
	w = h.do(http.MethodGet, "/api/v2/myrepo/blobs/uploads/"+uuid, nil, "")
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Push the full blob as a chunk.
	w = h.do(http.MethodPatch, "/api/v2/myrepo/blobs/uploads/"+uuid, blob, "application/octet-stream")
	require.Equal(t, http.StatusAccepted, w.Code)

	// Complete with digest verification (no extra body).
	w = h.do(http.MethodPut, "/api/v2/myrepo/blobs/uploads/"+uuid+"?digest="+digest, nil, "")
	require.Equal(t, http.StatusCreated, w.Code)

	// Blob exists (HEAD) and downloads (GET).
	w = h.do(http.MethodHead, "/api/v2/myrepo/blobs/"+digest, nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodGet, "/api/v2/myrepo/blobs/"+digest, nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, blob, w.Body.Bytes())

	// Uploader is recorded as owner on push, so they can delete the blob.
	w = h.do(http.MethodDelete, "/api/v2/myrepo/blobs/"+digest, nil, "")
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestOCIManifestPushPullTagsCatalogDelete(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	mf := dockerManifest(t)

	// Push manifest under tag "latest".
	w := h.do(http.MethodPut, "/api/v2/myrepo/manifests/latest", mf, dockerManifestType)
	require.Equal(t, http.StatusCreated, w.Code)

	// Pull manifest (GET) and HEAD.
	w = h.do(http.MethodGet, "/api/v2/myrepo/manifests/latest", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "schemaVersion")

	w = h.do(http.MethodHead, "/api/v2/myrepo/manifests/latest", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Pushed manifest is recorded, so it appears in tags/list and _catalog.
	w = h.do(http.MethodGet, "/api/v2/myrepo/tags/list", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "latest")

	w = h.do(http.MethodGet, "/api/v2/_catalog", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "myrepo")

	// Uploader is owner, so manifest delete is accepted.
	w = h.do(http.MethodDelete, "/api/v2/myrepo/manifests/latest", nil, "")
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestOCIBlobUploadCancel(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPost, "/api/v2/myrepo/blobs/uploads/", nil, "")
	require.Equal(t, http.StatusAccepted, w.Code)
	uuid := w.Header().Get("Docker-Upload-UUID")
	require.NotEmpty(t, uuid)

	w = h.do(http.MethodDelete, "/api/v2/myrepo/blobs/uploads/"+uuid, nil, "")
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestOCIBlobComplete_MissingDigest(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPost, "/api/v2/myrepo/blobs/uploads/", nil, "")
	require.Equal(t, http.StatusAccepted, w.Code)
	uuid := w.Header().Get("Docker-Upload-UUID")

	// Complete without digest query param.
	w = h.do(http.MethodPut, "/api/v2/myrepo/blobs/uploads/"+uuid, nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOCIBlobGet_BadDigest(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/v2/myrepo/blobs/notadigest", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOCIBlobAndManifest_NotFound(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	missing := blobDigest([]byte("nope"))
	w := h.do(http.MethodGet, "/api/v2/myrepo/blobs/"+missing, nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = h.do(http.MethodGet, "/api/v2/myrepo/manifests/ghost", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOCIUnauthorized(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/v2/myrepo/tags/list")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOCIDockerTokenRequiresBasicAuth(t *testing.T) {
	h := newHarness(t)
	OCIRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/v2/token")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = h.doNoAuth(http.MethodGet, "/api/v2/auth")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ociBasic issues a request against the root-mounted catch-all router with HTTP
// Basic Auth credentials (used by the Docker login flow).
func (h *testHarness) ociBasic(method, path, user, pass string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.SetBasicAuth(user, pass)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, r)
	return w
}

// TestOCICatchAllFlow exercises the root-level catch-all router (OCIRootRoutes),
// which Docker CLIs hit at /v2/* with multi-segment repository names.
func TestOCICatchAllFlow(t *testing.T) {
	h := newHarness(t)
	OCIRootRoutes(h.router, h.registry, h.auth)

	repo := "library/myapp"
	blob := []byte("catchall-layer")
	digest := blobDigest(blob)

	// Base endpoint.
	w := h.do(http.MethodGet, "/v2/", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lodestone OCI Registry")

	// Blob upload session.
	w = h.do(http.MethodPost, "/v2/"+repo+"/blobs/uploads/", nil, "")
	require.Equal(t, http.StatusAccepted, w.Code)
	uuid := w.Header().Get("Docker-Upload-UUID")
	require.NotEmpty(t, uuid)

	// Status, chunk, complete.
	w = h.do(http.MethodGet, "/v2/"+repo+"/blobs/uploads/"+uuid, nil, "")
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = h.do(http.MethodPatch, "/v2/"+repo+"/blobs/uploads/"+uuid, blob, "application/octet-stream")
	require.Equal(t, http.StatusAccepted, w.Code)

	w = h.do(http.MethodPut, "/v2/"+repo+"/blobs/uploads/"+uuid+"?digest="+digest, nil, "")
	require.Equal(t, http.StatusCreated, w.Code)

	// Blob HEAD + GET.
	w = h.do(http.MethodHead, "/v2/"+repo+"/blobs/"+digest, nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodGet, "/v2/"+repo+"/blobs/"+digest, nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, blob, w.Body.Bytes())

	// Manifest push + pull through the catch-all.
	w = h.do(http.MethodPut, "/v2/"+repo+"/manifests/latest", dockerManifest(t), dockerManifestType)
	require.Equal(t, http.StatusCreated, w.Code)

	w = h.do(http.MethodGet, "/v2/"+repo+"/manifests/latest", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Tags + catalog endpoints respond (empty, per the documented manifest-record bug).
	w = h.do(http.MethodGet, "/v2/"+repo+"/tags/list", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodGet, "/v2/_catalog", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Unknown sub-path → 404.
	w = h.do(http.MethodGet, "/v2/"+repo+"/bogus", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOCICatchAllDockerAuth(t *testing.T) {
	h := newHarness(t)
	OCIRootRoutes(h.router, h.registry, h.auth)

	// No credentials → challenge.
	w := h.do(http.MethodGet, "/v2/auth", nil, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = h.do(http.MethodGet, "/v2/token", nil, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Valid username/password (login fallback) issues a bearer token.
	w = h.ociBasic(http.MethodGet, "/v2/auth", "routeuser", "testpassword123")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")

	// Wrong password → 401.
	w = h.ociBasic(http.MethodGet, "/v2/auth", "routeuser", "wrong")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
