package registry

import (
	"regexp"
	"strings"

	"github.com/lgulliver/lodestone/pkg/types"
)

// Handler defines the interface for registry-specific handlers
type Handler interface {
	Upload(artifact *types.Artifact, content []byte) error
	Download(name, version string) (*types.Artifact, []byte, error)
	List(filter *types.ArtifactFilter) ([]*types.Artifact, error)
	Delete(name, version string) error
	Validate(artifact *types.Artifact, content []byte) error
	GetMetadata(content []byte) (map[string]interface{}, error)
	GenerateStoragePath(name, version string) string
}

// For backward compatibility
type RegistryHandler = Handler

// Helper functions for validation

// isValidNuGetPackageName validates NuGet package names
func isValidNuGetPackageName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}

	// NuGet package names should not start or end with dots
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return false
	}

	// Check for valid characters (letters, numbers, dots, underscores, hyphens)
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, name)
	return matched
}

// isValidSemver performs basic semantic version validation
func isValidSemver(version string) bool {
	if version == "" {
		return false
	}

	// Basic semver pattern: X.Y.Z with optional pre-release and build
	matched, _ := regexp.MatchString(`^[0-9]+\.[0-9]+\.[0-9]+`, version)
	return matched
}

// isValidNPMPackageName validates npm package names
func isValidNPMPackageName(name string) bool {
	if name == "" || len(name) > 214 {
		return false
	}

	// npm packages can start with @ for scoped packages
	if strings.HasPrefix(name, "@") {
		parts := strings.Split(name, "/")
		if len(parts) != 2 {
			return false
		}
		// Validate scope and package name separately
		return isValidNPMName(parts[0][1:]) && isValidNPMName(parts[1])
	}

	return isValidNPMName(name)
}

// isValidNPMName validates individual npm name components
func isValidNPMName(name string) bool {
	if name == "" {
		return false
	}

	// npm names are lowercase and can contain dots, hyphens, underscores
	matched, _ := regexp.MatchString(`^[a-z0-9._-]+$`, name)
	return matched
}

// isValidGoModulePath validates Go module paths
func isValidGoModulePath(path string) bool {
	if path == "" {
		return false
	}

	// Basic validation for Go module paths
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return false
	}

	// First part should be a domain
	domain := parts[0]
	return strings.Contains(domain, ".")
}

// isValidMavenCoordinates validates Maven coordinates
func isValidMavenCoordinates(name string) bool {
	if name == "" {
		return false
	}

	// Maven coordinates format: groupId:artifactId
	parts := strings.Split(name, ":")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// isValidHelmChartName validates Helm chart names
func isValidHelmChartName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}

	// Helm chart names follow DNS naming conventions
	matched, _ := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, name)
	return matched
}

// isValidCargoPackageName validates Cargo package names
func isValidCargoPackageName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}

	// Cargo package names: alphanumeric, underscores, hyphens
	matched, _ := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9_-]*$`, name)
	return matched
}

// isValidGemName validates RubyGems package names
func isValidGemName(name string) bool {
	if name == "" {
		return false
	}

	// Gem names: letters, numbers, underscores, hyphens, dots
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, name)
	return matched
}

// isValidOPABundleName validates OPA bundle names
func isValidOPABundleName(name string) bool {
	if name == "" {
		return false
	}

	// OPA bundle names: letters, numbers, underscores, hyphens, slashes
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._/-]+$`, name)
	return matched
}
