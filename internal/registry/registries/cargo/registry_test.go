package cargo

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

type mockStorage struct {
	data    map[string][]byte
	failPut bool
}

func newMockStorage() *mockStorage { return &mockStorage{data: make(map[string][]byte)} }

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
	assert.NotNil(t, New(newMockStorage(), nil))
}

func TestUpload(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		st := newMockStorage()
		r := New(st, nil)
		art := &types.Artifact{Name: "serde", Version: "1.0.0", StoragePath: "cargo/crates/serde/serde-1.0.0.crate"}
		require.NoError(t, r.Upload(context.Background(), art, []byte("crate")))
		assert.Equal(t, "application/gzip", art.ContentType)
		assert.Equal(t, []byte("crate"), st.data[art.StoragePath])
	})
	t.Run("storage failure wrapped", func(t *testing.T) {
		st := newMockStorage()
		st.failPut = true
		err := New(st, nil).Upload(context.Background(), &types.Artifact{StoragePath: "p"}, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store Cargo package")
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
		{"valid", &types.Artifact{Name: "serde"}, []byte("x"), ""},
		{"valid with underscore and dash", &types.Artifact{Name: "my_crate-2"}, []byte("x"), ""},
		{"empty content", &types.Artifact{Name: "serde"}, nil, "empty crate content"},
		{"invalid start with digit", &types.Artifact{Name: "1crate"}, []byte("x"), "invalid crate name format"},
		{"invalid char", &types.Artifact{Name: "bad name"}, []byte("x"), "invalid crate name format"},
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
	md, err := New(newMockStorage(), nil).GetMetadata([]byte("x"))
	require.NoError(t, err)
	assert.Equal(t, "cargo", md["format"])
	assert.Equal(t, "crate", md["type"])
}

func TestGenerateStoragePath(t *testing.T) {
	r := New(newMockStorage(), nil)
	assert.Equal(t, "cargo/crates/serde/serde-1.0.0.crate", r.GenerateStoragePath("serde", "1.0.0"))
}

func TestDeprecatedMethods(t *testing.T) {
	r := New(newMockStorage(), nil)
	_, _, err := r.Download("n", "v")
	assert.Error(t, err)
	_, err = r.List(&types.ArtifactFilter{})
	assert.Error(t, err)
	assert.Error(t, r.Delete("n", "v"))
}
