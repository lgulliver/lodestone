package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortVersions(t *testing.T) {
	in := []string{"1.0.0", "2.1.0", "1.5.0", "bogus", "2.0.0"}
	got := SortVersions(in)
	// Invalid skipped; latest first.
	assert.Equal(t, []string{"2.1.0", "2.0.0", "1.5.0", "1.0.0"}, got)
}

func TestSortVersionsAscending(t *testing.T) {
	got := SortVersionsAscending([]string{"2.0.0", "1.0.0", "1.5.0"})
	assert.Equal(t, []string{"1.0.0", "1.5.0", "2.0.0"}, got)
}

func TestGetLatestVersion(t *testing.T) {
	assert.Equal(t, "", GetLatestVersion(nil))
	assert.Equal(t, "3.0.0", GetLatestVersion([]string{"1.0.0", "3.0.0", "2.0.0"}))
	// All invalid → falls back to first original.
	assert.Equal(t, "notaver", GetLatestVersion([]string{"notaver", "alsobad"}))
}

func TestIsPrerelease(t *testing.T) {
	assert.True(t, IsPrerelease("1.0.0-beta.1"))
	assert.False(t, IsPrerelease("1.0.0"))
	assert.False(t, IsPrerelease("invalid")) // conservative false
}

func TestSortSemver(t *testing.T) {
	versions := []string{"2.0.0", "1.0.0", "1.5.0"}
	SortSemver(versions)
	assert.Equal(t, []string{"1.0.0", "1.5.0", "2.0.0"}, versions)
}

func TestCompareVersions(t *testing.T) {
	assert.Equal(t, -1, CompareVersions("1.0.0", "2.0.0"))
	assert.Equal(t, 0, CompareVersions("1.0.0", "1.0.0"))
	assert.Equal(t, 1, CompareVersions("2.0.0", "1.0.0"))
	assert.Equal(t, 2, CompareVersions("bad", "1.0.0"))
	assert.Equal(t, 2, CompareVersions("1.0.0", "bad"))
}
