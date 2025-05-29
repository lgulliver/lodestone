package utils

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword"

	hash, err := HashPassword(password, 10)
	if err != nil {
		t.Errorf("HashPassword() error = %v", err)
		return
	}

	if len(hash) == 0 {
		t.Error("HashPassword() returned empty hash")
	}

	// Test that the same password produces different hashes (salt)
	hash2, err := HashPassword(password, 10)
	if err != nil {
		t.Errorf("HashPassword() error = %v", err)
		return
	}

	if hash == hash2 {
		t.Error("HashPassword() should produce different hashes due to salt")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "testpassword"
	wrongPassword := "wrongpassword"

	hash, err := HashPassword(password, 10)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			want:     true,
		},
		{
			name:     "wrong password",
			password: wrongPassword,
			hash:     hash,
			want:     false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPassword(tt.password, tt.hash); got != tt.want {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizePackageName(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		registryType string
		want         string
	}{
		{
			name:         "npm uppercase to lowercase",
			input:        "MyPackage",
			registryType: "npm",
			want:         "mypackage",
		},
		{
			name:         "npm spaces to hyphens",
			input:        "my package",
			registryType: "npm",
			want:         "my-package",
		},
		{
			name:         "npm underscores to hyphens",
			input:        "my_package",
			registryType: "npm",
			want:         "my-package",
		},
		{
			name:         "npm mixed case and characters",
			input:        "My_Package Name",
			registryType: "npm",
			want:         "my-package-name",
		},
		{
			name:         "npm already clean",
			input:        "my-package",
			registryType: "npm",
			want:         "my-package",
		},
		{
			name:         "nuget preserve case",
			input:        "Lodestone.TestLibrary",
			registryType: "nuget",
			want:         "Lodestone.TestLibrary",
		},
		{
			name:         "nuget spaces to hyphens",
			input:        "My Package Name",
			registryType: "nuget",
			want:         "My-Package-Name",
		},
		{
			name:         "nuget underscores to hyphens",
			input:        "My_Package_Name",
			registryType: "nuget",
			want:         "My-Package-Name",
		},
		{
			name:         "maven preserve case",
			input:        "com.example.MyArtifact",
			registryType: "maven",
			want:         "com.example.MyArtifact",
		},
		{
			name:         "default registry lowercase",
			input:        "MyPackage",
			registryType: "cargo",
			want:         "mypackage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizePackageName(tt.input, tt.registryType); got != tt.want {
				t.Errorf("SanitizePackageName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "valid version",
			version: "1.0.0",
			want:    true,
		},
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
		{
			name:    "long version",
			version: "this-is-a-very-long-version-string-that-exceeds-fifty-characters-limit",
			want:    false,
		},
		{
			name:    "simple version",
			version: "v1",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateVersion(tt.version); got != tt.want {
				t.Errorf("ValidateVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidRegistryType(t *testing.T) {
	tests := []struct {
		name         string
		registryType string
		want         bool
	}{
		{
			name:         "valid nuget",
			registryType: "nuget",
			want:         true,
		},
		{
			name:         "valid npm",
			registryType: "npm",
			want:         true,
		},
		{
			name:         "valid cargo",
			registryType: "cargo",
			want:         true,
		},
		{
			name:         "invalid type",
			registryType: "invalid",
			want:         false,
		},
		{
			name:         "empty type",
			registryType: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidRegistryType(tt.registryType); got != tt.want {
				t.Errorf("IsValidRegistryType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "bytes",
			bytes: 512,
			want:  "512 B",
		},
		{
			name:  "kilobytes",
			bytes: 1536, // 1.5 KB
			want:  "1.5 KB",
		},
		{
			name:  "megabytes",
			bytes: 1048576, // 1 MB
			want:  "1.0 MB",
		},
		{
			name:  "zero bytes",
			bytes: 0,
			want:  "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBytes(tt.bytes); got != tt.want {
				t.Errorf("FormatBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
