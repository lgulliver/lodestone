package nuget

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

func buildNupkg(t *testing.T, nuspec string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("mypkg.nuspec")
	require.NoError(t, err)
	_, err = w.Write([]byte(nuspec))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestGetMetadata_FullNuspec(t *testing.T) {
	r := New(nil, nil)
	nuspec := `<?xml version="1.0"?>
<package>
  <metadata>
    <id>MyPkg</id>
    <version>1.2.3</version>
    <title>My Package</title>
    <authors>Alice,Bob</authors>
    <owners>Org</owners>
    <description>a desc</description>
    <summary>a summary</summary>
    <tags>web api</tags>
    <projectUrl>http://proj</projectUrl>
    <licenseUrl>http://lic</licenseUrl>
    <iconUrl>http://icon</iconUrl>
    <copyright>2026</copyright>
    <language>en-US</language>
  </metadata>
</package>`

	meta, err := r.GetMetadata(buildNupkg(t, nuspec))
	require.NoError(t, err)
	assert.Equal(t, "MyPkg", meta["id"])
	assert.Equal(t, "1.2.3", meta["version"])
	assert.Equal(t, "a desc", meta["description"])
	assert.Equal(t, "a summary", meta["summary"])
	assert.Equal(t, []string{"Alice", "Bob"}, meta["authors"])
	assert.Equal(t, []string{"Org"}, meta["owners"])
	assert.Equal(t, []string{"web", "api"}, meta["tags"])
	assert.Equal(t, "http://proj", meta["projectUrl"])
	assert.Equal(t, "2026", meta["copyright"])
}

func TestGetMetadata_RichNuspec(t *testing.T) {
	r := New(nil, nil)
	nuspec := `<?xml version="1.0"?>
<package>
  <metadata>
    <id>Rich</id>
    <version>2.0.0</version>
    <title>Rich Pkg</title>
    <authors>A</authors>
    <description>d</description>
    <minClientVersion>3.3.0</minClientVersion>
    <releaseNotes>notes</releaseNotes>
    <requireLicenseAcceptance>true</requireLicenseAcceptance>
    <developmentDependency>true</developmentDependency>
    <license type="expression">MIT</license>
    <repository type="git" url="http://repo" branch="main" commit="abc"/>
    <packageTypes>
      <packageType name="Dependency" version="1.0.0"/>
    </packageTypes>
    <frameworkAssemblies>
      <frameworkAssembly assemblyName="System.Net" targetFramework="net6.0"/>
    </frameworkAssemblies>
    <dependencies>
      <group targetFramework="net6.0">
        <dependency id="Newtonsoft.Json" version="13.0.0" include="all" exclude="none"/>
      </group>
    </dependencies>
  </metadata>
</package>`

	meta, err := r.GetMetadata(buildNupkg(t, nuspec))
	require.NoError(t, err)
	assert.Equal(t, "Rich", meta["id"])
	assert.Equal(t, "notes", meta["releaseNotes"])
	assert.Equal(t, true, meta["requireLicenseAcceptance"])
	assert.Equal(t, true, meta["developmentDependency"])
	assert.Equal(t, "3.3.0", meta["minClientVersion"])
	assert.Contains(t, meta, "license")
	assert.Contains(t, meta, "repository")
	assert.Contains(t, meta, "packageTypes")
	assert.Contains(t, meta, "frameworkAssemblies")
	assert.Contains(t, meta, "dependencyGroups")
}

func TestGetMetadata_FlatDependencies(t *testing.T) {
	r := New(nil, nil)
	nuspec := `<?xml version="1.0"?>
<package>
  <metadata>
    <id>Flat</id>
    <version>1.0.0</version>
    <authors>A</authors>
    <description>d</description>
    <dependencies>
      <dependency id="left-pad" version="1.0.0"/>
    </dependencies>
  </metadata>
</package>`

	meta, err := r.GetMetadata(buildNupkg(t, nuspec))
	require.NoError(t, err)
	assert.Contains(t, meta, "dependencies")
}

func TestGetMetadata_InvalidZipFallsBackToBasic(t *testing.T) {
	r := New(nil, nil)
	meta, err := r.GetMetadata([]byte("not a zip"))
	require.NoError(t, err)
	assert.Equal(t, "nuget", meta["format"])
	assert.NotContains(t, meta, "id")
}

func TestExtractNuspec_NotFound(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	_, _ = w.Write([]byte("hi"))
	require.NoError(t, zw.Close())

	_, err := extractNuspecFromNupkg(buf.Bytes())
	assert.ErrorContains(t, err, ".nuspec file not found")
}

func TestIsSymbolPackage_Branches(t *testing.T) {
	r := New(nil, nil)

	assert.True(t, r.IsSymbolPackage(&types.Artifact{Metadata: map[string]interface{}{"packageType": "symbols"}}))
	assert.True(t, r.IsSymbolPackage(&types.Artifact{Metadata: map[string]interface{}{"contentType": "application/vnd.nuget.symbolpackage"}}))
	assert.True(t, r.IsSymbolPackage(&types.Artifact{ContentType: "application/vnd.nuget.symbolpackage"}))
	assert.False(t, r.IsSymbolPackage(&types.Artifact{ContentType: "application/zip"}))
	assert.False(t, r.IsSymbolPackage(&types.Artifact{Metadata: map[string]interface{}{"packageType": "regular"}}))
}
