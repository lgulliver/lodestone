package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMavenUpload_Created(t *testing.T) {
	h := newHarness(t)
	MavenRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPut, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar",
		[]byte("jar-bytes"), "application/java-archive")
	assert.Equal(t, http.StatusCreated, w.Code)
}

// Download/HEAD/Delete resolve packageName as "<group>:<artifactId>" where
// group = join(parts[:len-3]) and artifactId = parts[len-3]. We seed under that
// exact name so the read path can find it.
func TestMavenDownloadHeadDelete(t *testing.T) {
	h := newHarness(t)
	MavenRoutes(h.api, h.registry, h.auth)
	h.seedArtifact(t, "maven", "com.example:artifact", "1.0.0", []byte("jar-bytes"), nil)

	w := h.do(http.MethodGet, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "jar-bytes", w.Body.String())

	w = h.do(http.MethodHead, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = h.do(http.MethodDelete, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar", nil, "")
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// BUG: upload builds the package name as join(parts[:len-2]) + ":" + artifactID,
// which folds the artifactId into the groupId (-> "com.example.artifact:artifact").
// Download/HEAD/Delete instead use join(parts[:len-3]) + ":" + parts[len-3]
// (-> "com.example:artifact"). The two names never match, so an artifact uploaded
// through the Maven PUT endpoint can never be fetched back through the GET endpoint.
// Documented here until the path-parsing in handleMavenUpload is aligned with the
// read handlers.
func TestMavenUploadThenDownload_NameMismatchBug(t *testing.T) {
	h := newHarness(t)
	MavenRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPut, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar",
		[]byte("jar-bytes"), "application/java-archive")
	assert.Equal(t, http.StatusCreated, w.Code)

	w = h.do(http.MethodGet, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMavenDownload_NotFound(t *testing.T) {
	h := newHarness(t)
	MavenRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/maven/com/example/ghost/9.9.9/ghost-9.9.9.jar", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = h.do(http.MethodHead, "/api/maven/com/example/ghost/9.9.9/ghost-9.9.9.jar", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMavenPathTooShort(t *testing.T) {
	h := newHarness(t)
	MavenRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/maven/a/b/c", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = h.do(http.MethodPut, "/api/maven/a/b/c", []byte("x"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = h.do(http.MethodDelete, "/api/maven/a/b/c", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = h.do(http.MethodHead, "/api/maven/a/b/c", nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMavenUnauthorized(t *testing.T) {
	h := newHarness(t)
	MavenRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/maven/com/example/artifact/1.0.0/artifact-1.0.0.jar")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
