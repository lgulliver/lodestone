package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a password against its hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateAPIKey generates a random API key
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashAPIKey hashes an API key for storage
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// GenerateJWT generates a JWT token
func GenerateJWT(userID uuid.UUID, secret string, expiration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(expiration).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT validates and parses a JWT token
func ValidateJWT(tokenString, secret string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			return uuid.Nil, fmt.Errorf("invalid user_id claim")
		}
		
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid user_id format")
		}
		
		return userID, nil
	}

	return uuid.Nil, fmt.Errorf("invalid token")
}

// ComputeSHA256 computes the SHA256 hash of data
func ComputeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ComputeSHA256FromReader computes SHA256 hash from an io.Reader
func ComputeSHA256FromReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SanitizePackageName sanitizes a package name for safe storage
func SanitizePackageName(name string) string {
	// Convert to lowercase and replace invalid characters
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// ValidateVersion checks if a version string is valid (basic semver)
func ValidateVersion(version string) bool {
	// Basic validation - should be enhanced for specific registry requirements
	if version == "" {
		return false
	}
	
	// Check for common version patterns
	return len(version) > 0 && len(version) <= 50
}

// IsValidRegistryType checks if a registry type is supported
func IsValidRegistryType(registryType string) bool {
	validTypes := []string{
		"nuget", "oci", "opa", "maven", "npm", 
		"cargo", "go", "helm", "rubygems",
	}
	
	for _, valid := range validTypes {
		if registryType == valid {
			return true
		}
	}
	return false
}

// FormatBytes formats byte size in human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	suffixes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffixes[exp])
}
