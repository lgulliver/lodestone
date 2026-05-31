package routes

import (
	"archive/zip"
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nupkgBytes builds a minimal .nupkg/.snupkg zip containing an <id>.nuspec file.
func nupkgBytes(t *testing.T, id, version string) []byte {
	t.Helper()
	nuspec := `<?xml version="1.0"?>
<package><metadata><id>` + id + `</id><version>` + version + `</version>` +
		`<authors>Alice</authors><description>a package</description></metadata></package>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(id + ".nuspec")
	require.NoError(t, err)
	_, err = f.Write([]byte(nuspec))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// snupkgBytes builds a symbol package: nuspec + a .pdb entry (required by Validate).
func snupkgBytes(t *testing.T, id, version string) []byte {
	t.Helper()
	nuspec := `<?xml version="1.0"?>
<package><metadata><id>` + id + `</id><version>` + version + `</version>` +
		`<authors>Alice</authors><description>a package</description></metadata></package>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	nf, err := zw.Create(id + ".nuspec")
	require.NoError(t, err)
	_, err = nf.Write([]byte(nuspec))
	require.NoError(t, err)
	pf, err := zw.Create("lib/" + id + ".pdb")
	require.NoError(t, err)
	_, err = pf.Write([]byte("fake-symbols"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestNuGetUploadVersionsDownloadSearchMetadataDelete(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	// Upload (raw binary, NuGet CLI style).
	w := h.do(http.MethodPut, "/api/nuget/v2/package",
		nupkgBytes(t, "MyPkg", "1.0.0"), "application/octet-stream")
	require.Equal(t, http.StatusCreated, w.Code)

	// Versions list.
	w = h.do(http.MethodGet, "/api/nuget/v3-flatcontainer/MyPkg/index.json", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1.0.0")

	// Download.
	w = h.do(http.MethodGet, "/api/nuget/v3-flatcontainer/MyPkg/1.0.0/MyPkg.1.0.0.nupkg", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Search.
	w = h.do(http.MethodGet, "/api/nuget/v3/search?q=MyPkg", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "MyPkg")

	// Registration metadata.
	w = h.do(http.MethodGet, "/api/nuget/v3/registration/MyPkg/index.json", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "1.0.0")

	// Delete (exact version).
	w = h.do(http.MethodDelete, "/api/nuget/v2/package/MyPkg/1.0.0", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNuGetServiceIndex(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/nuget/v3/index.json", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PackagePublish")
}

func TestNuGetSymbolUploadDownload(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	// Base package must exist before symbols.
	w := h.do(http.MethodPut, "/api/nuget/v2/package",
		nupkgBytes(t, "MyPkg", "1.0.0"), "application/octet-stream")
	require.Equal(t, http.StatusCreated, w.Code)

	// Symbol upload (raw).
	w = h.do(http.MethodPut, "/api/nuget/v2/symbolpackage",
		snupkgBytes(t, "MyPkg", "1.0.0"), "application/octet-stream")
	require.Equal(t, http.StatusCreated, w.Code)

	// Symbol download.
	w = h.do(http.MethodGet, "/api/nuget/symbols/MyPkg/1.0.0/MyPkg.1.0.0.snupkg", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNuGetSymbol_NoBasePackage(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPut, "/api/nuget/v2/symbolpackage",
		nupkgBytes(t, "Ghost", "1.0.0"), "application/octet-stream")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "base package")
}

func TestNuGetUpload_InvalidPackage(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodPut, "/api/nuget/v2/package",
		[]byte("not-a-zip"), "application/octet-stream")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNuGetDownloadMetadata_NotFound(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	w := h.do(http.MethodGet, "/api/nuget/v3-flatcontainer/Ghost/9.9.9/Ghost.9.9.9.nupkg", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = h.do(http.MethodGet, "/api/nuget/v3/registration/Ghost/index.json", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNuGetUnauthorized(t *testing.T) {
	h := newHarness(t)
	NuGetRoutes(h.api, h.registry, h.auth)

	w := h.doNoAuth(http.MethodGet, "/api/nuget/v3-flatcontainer/MyPkg/1.0.0/MyPkg.1.0.0.nupkg")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
