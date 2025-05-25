package auth

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	tests := []struct {
		name string
		want int // number of keys to generate for testing
	}{
		{
			name: "generate multiple keys",
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generatedKeys := make(map[string]bool)

			for i := 0; i < tt.want; i++ {
				key, err := GenerateAPIKey()
				if err != nil {
					t.Errorf("GenerateAPIKey() error = %v", err)
					return
				}

				// Check format
				if !ValidateAPIKeyFormat(key) {
					t.Errorf("GenerateAPIKey() generated invalid format: %s", key)
				}

				// Check uniqueness
				if generatedKeys[key] {
					t.Errorf("GenerateAPIKey() generated duplicate key: %s", key)
				}
				generatedKeys[key] = true

				// Check structure
				parts := strings.Split(key, "-")
				if len(parts) != 6 {
					t.Errorf("GenerateAPIKey() generated key with wrong structure: %s", key)
				}

				// Check hex component (should be 24 uppercase hex chars)
				hexPart := parts[4]
				if len(hexPart) != 24 {
					t.Errorf("GenerateAPIKey() generated key with wrong hex length: %s", hexPart)
				}

				hexPattern := regexp.MustCompile(`^[A-F0-9]{24}$`)
				if !hexPattern.MatchString(hexPart) {
					t.Errorf("GenerateAPIKey() generated key with invalid hex component: %s", hexPart)
				}
			}
		})
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	// Generate a valid key for testing
	validKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	tests := []struct {
		name   string
		apiKey string
		want   bool
	}{
		{
			name:   "valid generated key",
			apiKey: validKey,
			want:   true,
		},
		{
			name:   "valid example key",
			apiKey: "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime",
			want:   true,
		},
		{
			name:   "empty string",
			apiKey: "",
			want:   false,
		},
		{
			name:   "too few parts",
			apiKey: "north-quantum-dragon",
			want:   false,
		},
		{
			name:   "too many parts",
			apiKey: "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime-extra",
			want:   false,
		},
		{
			name:   "invalid prefix",
			apiKey: "invalid-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime",
			want:   false,
		},
		{
			name:   "invalid adjective",
			apiKey: "north-invalid-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime",
			want:   false,
		},
		{
			name:   "invalid noun",
			apiKey: "north-quantum-invalid-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime",
			want:   false,
		},
		{
			name:   "invalid suffix",
			apiKey: "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-seven",
			want:   false,
		},
		{
			name:   "hex too short",
			apiKey: "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1-prime",
			want:   false,
		},
		{
			name:   "hex too long",
			apiKey: "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2A3-prime",
			want:   false,
		},
		{
			name:   "lowercase hex",
			apiKey: "north-quantum-dragon-neural-a1b2c3d4e5f6a7b8c9d0e1f2-prime",
			want:   false,
		},
		{
			name:   "non-hex characters",
			apiKey: "north-quantum-dragon-neural-G1H2I3J4K5L6M7N8O9P0Q1R2-prime",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAPIKeyFormat(tt.apiKey); got != tt.want {
				t.Errorf("ValidateAPIKeyFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAPIKeyFormat(t *testing.T) {
	// Generate a valid new key for testing
	validKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	tests := []struct {
		name   string
		apiKey string
		want   string
	}{
		{
			name:   "human-readable generated key",
			apiKey: validKey,
			want:   "human-readable",
		},
		{
			name:   "human-readable example key",
			apiKey: "north-quantum-dragon-neural-A1B2C3D4E5F6071829A0B1C2-prime",
			want:   "human-readable",
		},
		{
			name:   "invalid key",
			apiKey: "invalid-key-format",
			want:   "invalid",
		},
		{
			name:   "empty string",
			apiKey: "",
			want:   "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAPIKeyFormat(tt.apiKey); got != tt.want {
				t.Errorf("GetAPIKeyFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "human-readable key",
			key:  "north-quantum-dragon-neural-A1B2C3D4E5F6071829A0B1C2-prime",
		},
		{
			name: "empty string",
			key:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := HashAPIKey(tt.key)
			hash2 := HashAPIKey(tt.key)

			// Hash should be consistent
			if hash1 != hash2 {
				t.Errorf("HashAPIKey() produced inconsistent results: %s != %s", hash1, hash2)
			}

			// Hash should be 64 characters (SHA256 hex)
			if len(hash1) != 64 {
				t.Errorf("HashAPIKey() produced hash with wrong length: %d", len(hash1))
			}

			// Hash should be lowercase hex
			hexPattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
			if !hexPattern.MatchString(hash1) {
				t.Errorf("HashAPIKey() produced invalid hash format: %s", hash1)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	testSlice := []string{"apple", "banana", "cherry"}

	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "item exists",
			slice: testSlice,
			item:  "banana",
			want:  true,
		},
		{
			name:  "item does not exist",
			slice: testSlice,
			item:  "grape",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			item:  "apple",
			want:  false,
		},
		{
			name:  "empty item",
			slice: testSlice,
			item:  "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsString(tt.slice, tt.item); got != tt.want {
				t.Errorf("containsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGenerateAPIKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateAPIKey()
		if err != nil {
			b.Fatalf("GenerateAPIKey() error = %v", err)
		}
	}
}

func BenchmarkValidateAPIKeyFormat(b *testing.B) {
	key := "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ValidateAPIKeyFormat(key)
	}
}

func BenchmarkHashAPIKey(b *testing.B) {
	key := "north-quantum-dragon-neural-A1B2C3D4E5F6A7B8C9D0E1F2-prime"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HashAPIKey(key)
	}
}
