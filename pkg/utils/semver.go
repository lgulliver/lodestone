package utils

import (
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/rs/zerolog/log"
)

// SortVersions sorts the given version strings in semantic versioning order (latest version first)
func SortVersions(versions []string) []string {
	// Parse versions to semver.Version objects
	semverVersions := make([]*semver.Version, 0, len(versions))

	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			// Log and skip invalid versions
			log.Warn().Str("version", v).Err(err).Msg("invalid semver version")
			continue
		}
		semverVersions = append(semverVersions, sv)
	}

	// Sort versions (latest first)
	sort.Slice(semverVersions, func(i, j int) bool {
		return semverVersions[i].GreaterThan(semverVersions[j])
	})

	// Convert back to string
	result := make([]string, len(semverVersions))
	for i, v := range semverVersions {
		result[i] = v.String()
	}

	return result
}

// SortVersionsAscending sorts the given version strings in ascending semantic versioning order (oldest first)
func SortVersionsAscending(versions []string) []string {
	// Parse versions to semver.Version objects
	semverVersions := make([]*semver.Version, 0, len(versions))

	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			// Log and skip invalid versions
			log.Warn().Str("version", v).Err(err).Msg("invalid semver version")
			continue
		}
		semverVersions = append(semverVersions, sv)
	}

	// Sort versions (oldest first)
	sort.Slice(semverVersions, func(i, j int) bool {
		return semverVersions[i].LessThan(semverVersions[j])
	})

	// Convert back to string
	result := make([]string, len(semverVersions))
	for i, v := range semverVersions {
		result[i] = v.String()
	}

	return result
}

// GetLatestVersion returns the latest version from the given version strings
func GetLatestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}

	sortedVersions := SortVersions(versions)
	if len(sortedVersions) == 0 {
		// If no valid versions, return the original first one
		return versions[0]
	}

	return sortedVersions[0]
}

// IsPrerelease checks if a version is a prerelease version (e.g., beta, alpha, rc)
func IsPrerelease(version string) bool {
	sv, err := semver.NewVersion(version)
	if err != nil {
		// If invalid semver, conservatively return false
		return false
	}

	return sv.Prerelease() != ""
}

// SortSemver sorts the given version strings in ascending semantic versioning order (oldest first)
// Note: This modifies the original slice in-place rather than returning a new slice
func SortSemver(versions []string) {
	// Parse versions to semver.Version objects
	semverMap := make(map[string]*semver.Version, len(versions))
	validVersions := make([]*semver.Version, 0, len(versions))

	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			// Log and keep invalid versions without sorting them
			log.Warn().Str("version", v).Err(err).Msg("invalid semver version")
			continue
		}
		validVersions = append(validVersions, sv)
		semverMap[sv.String()] = sv
	}

	// Sort versions (oldest first)
	sort.Slice(validVersions, func(i, j int) bool {
		return validVersions[i].LessThan(validVersions[j])
	})

	// Update original slice with sorted versions
	for i := 0; i < len(validVersions); i++ {
		if i < len(versions) {
			versions[i] = validVersions[i].String()
		}
	}
}

// CompareVersions compares two version strings according to semver rules
// Returns:
//   -1 if v1 < v2
//    0 if v1 == v2
//    1 if v1 > v2
//    2 if either version is invalid
func CompareVersions(v1, v2 string) int {
	sv1, err := semver.NewVersion(v1)
	if err != nil {
		return 2
	}

	sv2, err := semver.NewVersion(v2)
	if err != nil {
		return 2
	}

	if sv1.LessThan(sv2) {
		return -1
	}
	if sv1.Equal(sv2) {
		return 0
	}
	return 1
}
