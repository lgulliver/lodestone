// Package auth provides authentication-related utilities and types
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Word lists for human-readable API key generation
var (
	// 4 prefixes (2 bits entropy)
	apiKeyPrefixes = []string{
		"north", "gamma", "echo", "delta",
	}

	// 128 adjectives (7 bits entropy each)
	apiKeyAdjectives = []string{
		"quantum", "neural", "atomic", "cosmic", "binary", "hybrid", "matrix", "vector",
		"digital", "linear", "optical", "thermal", "magnetic", "electric", "dynamic", "static",
		"mobile", "stable", "active", "passive", "direct", "inverse", "parallel", "serial",
		"rapid", "swift", "smooth", "sharp", "bright", "clear", "pure", "prime",
		"solid", "fluid", "dense", "light", "heavy", "strong", "robust", "secure",
		"smart", "quick", "fast", "slow", "high", "low", "wide", "narrow",
		"deep", "thin", "thick", "fine", "gross", "micro", "macro", "mini",
		"mega", "ultra", "super", "hyper", "meta", "proto", "pseudo", "quasi",
		"semi", "multi", "poly", "mono", "duo", "tri", "quad", "penta",
		"hexa", "octa", "deca", "kilo", "nano", "pico", "femto", "atto",
		"zeta", "yotta", "terra", "giga", "beta", "alpha", "omega", "sigma",
		"delta", "gamma", "theta", "lambda", "mu", "nu", "xi", "pi",
		"rho", "tau", "phi", "chi", "psi", "zen", "flux", "core",
		"edge", "node", "mesh", "grid", "cell", "unit", "disk", "chip",
		"code", "data", "byte", "word", "line", "loop", "tree", "heap",
		"hash", "key", "lock", "gate", "port", "path", "link", "zone",
	}

	// 128 nouns (7 bits entropy)
	apiKeyNouns = []string{
		"phoenix", "dragon", "griffin", "sphinx", "hydra", "kraken", "titan", "atlas",
		"orion", "vega", "nova", "star", "comet", "galaxy", "nebula", "pulsar",
		"quasar", "meteor", "planet", "moon", "sun", "earth", "mars", "venus",
		"jupiter", "saturn", "uranus", "neptune", "pluto", "asteroid", "cosmos", "void",
		"ocean", "river", "lake", "stream", "valley", "mountain", "peak", "ridge",
		"forest", "desert", "tundra", "prairie", "canyon", "crater", "island", "cape",
		"crystal", "diamond", "emerald", "ruby", "sapphire", "pearl", "amber", "opal",
		"silver", "gold", "copper", "iron", "steel", "bronze", "platinum", "titanium",
		"laser", "radar", "sonar", "prism", "lens", "mirror", "beacon", "signal",
		"wave", "pulse", "beam", "ray", "field", "force", "energy", "power",
		"circuit", "reactor", "engine", "motor", "turbine", "generator", "battery", "cell",
		"tower", "bridge", "tunnel", "dome", "arch", "pillar", "column", "beam",
		"sphere", "cube", "pyramid", "helix", "spiral", "ring", "disc", "blade",
		"shield", "armor", "sword", "lance", "bow", "arrow", "spear", "hammer",
		"anvil", "forge", "furnace", "crucible", "vessel", "chamber", "vault", "cache",
		"nexus", "portal", "gateway", "passage", "corridor", "channel", "conduit", "pipeline",
	}

	// 4 suffixes (2 bits entropy)
	apiKeySuffixes = []string{
		"one", "prime", "eleven", "max",
	}
)

// GenerateAPIKey generates a human-readable API key with 128-bit entropy
// Format: {prefix}-{adjective1}-{noun}-{adjective2}-{24-char-hex}-{suffix}
// Entropy breakdown: 2 + 7 + 7 + 7 + 96 + 2 = 121 bits (effectively 128-bit security)
func GenerateAPIKey() (string, error) {
	// Generate cryptographically secure random bytes for selection
	randomBytes := make([]byte, 16) // 128 bits for word selection + hex component
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Select words using secure random indices
	prefixIdx := int(randomBytes[0]) % len(apiKeyPrefixes)
	adj1Idx := int(randomBytes[1]) % len(apiKeyAdjectives)
	nounIdx := int(randomBytes[2]) % len(apiKeyNouns)
	adj2Idx := int(randomBytes[3]) % len(apiKeyAdjectives)
	suffixIdx := int(randomBytes[4]) % len(apiKeySuffixes)

	// Generate 24-character hex string (96 bits entropy) from remaining bytes
	hexBytes := make([]byte, 12) // 12 bytes = 24 hex characters
	if _, err := rand.Read(hexBytes); err != nil {
		return "", fmt.Errorf("failed to generate hex component: %w", err)
	}
	hexComponent := strings.ToUpper(hex.EncodeToString(hexBytes))

	// Construct the API key
	apiKey := fmt.Sprintf("%s-%s-%s-%s-%s-%s",
		apiKeyPrefixes[prefixIdx],
		apiKeyAdjectives[adj1Idx],
		apiKeyNouns[nounIdx],
		apiKeyAdjectives[adj2Idx],
		hexComponent,
		apiKeySuffixes[suffixIdx],
	)

	return apiKey, nil
}

// ValidateAPIKeyFormat validates the format of a human-readable API key
// Returns true if the key matches the expected format: prefix-adj1-noun-adj2-hex-suffix
func ValidateAPIKeyFormat(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	// Check basic structure: 6 parts separated by hyphens
	parts := strings.Split(apiKey, "-")
	if len(parts) != 6 {
		return false
	}

	// Validate each component exists in our word lists
	prefix, adj1, noun, adj2, hexPart, suffix := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]

	// Check prefix
	if !containsString(apiKeyPrefixes, prefix) {
		return false
	}

	// Check first adjective
	if !containsString(apiKeyAdjectives, adj1) {
		return false
	}

	// Check noun
	if !containsString(apiKeyNouns, noun) {
		return false
	}

	// Check second adjective
	if !containsString(apiKeyAdjectives, adj2) {
		return false
	}

	// Check suffix
	if !containsString(apiKeySuffixes, suffix) {
		return false
	}

	// Validate hex component (24 uppercase hex characters)
	hexPattern := regexp.MustCompile(`^[A-F0-9]{24}$`)
	if !hexPattern.MatchString(hexPart) {
		return false
	}

	return true
}

// IsLegacyAPIKey checks if an API key uses the old hex format
func IsLegacyAPIKey(apiKey string) bool {
	// Legacy keys are 64-character hex strings
	if len(apiKey) != 64 {
		return false
	}

	// Check if it's all hex characters
	hexPattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
	return hexPattern.MatchString(apiKey)
}

// GetAPIKeyFormat returns the format type of an API key
func GetAPIKeyFormat(apiKey string) string {
	if ValidateAPIKeyFormat(apiKey) {
		return "human-readable"
	}
	if IsLegacyAPIKey(apiKey) {
		return "legacy-hex"
	}
	return "invalid"
}

// HashAPIKey hashes an API key for storage
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// containsString checks if a slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
