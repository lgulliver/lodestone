package goregistry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

// mockStorage is an in-memory BlobStorage for tests.
type mockStorage struct {
	data    map[string][]byte
	failPut bool
}

func newMockStorage() *mockStorage {
	return &mockStorage{data: make(map[string][]byte)}
}

func (m *mockStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	if m.failPut {
		return fmt.Errorf("store failed")
	}
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	m.data[path] = data
	return nil
}

func (m *mockStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	if data, ok := m.data[path]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, io.EOF
}

func (m *mockStorage) Delete(ctx context.Context, path string) error {
	delete(m.data, path)
	return nil
}

func (m *mockStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, ok := m.data[path]
	return ok, nil
}

func (m *mockStorage) GetSize(ctx context.Context, path string) (int64, error) {
	if data, ok := m.data[path]; ok {
		return int64(len(data)), nil
	}
	return 0, io.EOF
}

func (m *mockStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var out []string
	for p := range m.data {
		if len(prefix) == 0 || (len(p) >= len(prefix) && p[:len(prefix)] == prefix) {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestNew(t *testing.T) {
	r := New(newMockStorage(), nil)
	assert.NotNil(t, r)
}

func TestUpload(t *testing.T) {
	t.Run("happy path stores content and sets content type", func(t *testing.T) {
		st := newMockStorage()
		r := New(st, nil)
		art := &types.Artifact{Name: "example.com/mod", Version: "v1.0.0", StoragePath: "go/example.com/mod/@v/v1.0.0.zip"}
		err := r.Upload(context.Background(), art, []byte("zipdata"))
		require.NoError(t, err)
		assert.Equal(t, "application/zip", art.ContentType)
		assert.Equal(t, []byte("zipdata"), st.data[art.StoragePath])
	})

	t.Run("storage failure is wrapped", func(t *testing.T) {
		st := newMockStorage()
		st.failPut = true
		r := New(st, nil)
		art := &types.Artifact{Name: "m", Version: "v1.0.0", StoragePath: "p"}
		err := r.Upload(context.Background(), art, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store Go module")
	})
}

func TestValidate(t *testing.T) {
	r := New(newMockStorage(), nil)
	tests := []struct {
		name    string
		art     *types.Artifact
		content []byte
		wantErr string
	}{
		{"valid module", &types.Artifact{Name: "github.com/foo/bar", Version: "v1.2.3"}, []byte("x"), ""},
		{"valid prerelease", &types.Artifact{Name: "example.com/x", Version: "v1.0.0-beta.1"}, []byte("x"), ""},
		{"valid build metadata", &types.Artifact{Name: "example.com/x", Version: "v1.0.0+build.5"}, []byte("x"), ""},
		{"empty content", &types.Artifact{Name: "example.com/x", Version: "v1.0.0"}, nil, "empty module content"},
		{"invalid module path uppercase", &types.Artifact{Name: "Example.com/X", Version: "v1.0.0"}, []byte("x"), "invalid Go module path format"},
		{"invalid version missing v", &types.Artifact{Name: "example.com/x", Version: "1.0.0"}, []byte("x"), "invalid semantic version format"},
		{"invalid version partial", &types.Artifact{Name: "example.com/x", Version: "v1.0"}, []byte("x"), "invalid semantic version format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Validate(tt.art, tt.content)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetMetadata(t *testing.T) {
	r := New(newMockStorage(), nil)
	md, err := r.GetMetadata([]byte("anything"))
	require.NoError(t, err)
	assert.Equal(t, "go", md["format"])
	assert.Equal(t, "module", md["type"])
}

func TestGenerateStoragePath(t *testing.T) {
	r := New(newMockStorage(), nil)
	assert.Equal(t, "go/example.com/mod/@v/v1.0.0.zip", r.GenerateStoragePath("example.com/mod", "v1.0.0"))
}

func TestDeprecatedMethods(t *testing.T) {
	r := New(newMockStorage(), nil)
	_, _, err := r.Download("n", "v")
	assert.Error(t, err)
	_, err = r.List(&types.ArtifactFilter{})
	assert.Error(t, err)
	assert.Error(t, r.Delete("n", "v"))
}
