package helm

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
		art := &types.Artifact{Name: "nginx", Version: "1.0.0", StoragePath: "helm/charts/nginx-1.0.0.tgz"}
		require.NoError(t, r.Upload(context.Background(), art, []byte("tgz")))
		assert.Equal(t, "application/gzip", art.ContentType)
		assert.Equal(t, []byte("tgz"), st.data[art.StoragePath])
	})
	t.Run("storage failure wrapped", func(t *testing.T) {
		st := newMockStorage()
		st.failPut = true
		err := New(st, nil).Upload(context.Background(), &types.Artifact{StoragePath: "p"}, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store Helm chart")
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
		{"valid single char", &types.Artifact{Name: "a"}, []byte("x"), ""},
		{"valid with dashes", &types.Artifact{Name: "my-chart-1"}, []byte("x"), ""},
		{"empty content", &types.Artifact{Name: "nginx"}, nil, "empty chart content"},
		{"invalid uppercase", &types.Artifact{Name: "Nginx"}, []byte("x"), "invalid Helm chart name format"},
		{"invalid trailing dash", &types.Artifact{Name: "nginx-"}, []byte("x"), "invalid Helm chart name format"},
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
	assert.Equal(t, "helm", md["format"])
	assert.Equal(t, "chart", md["type"])
}

func TestGenerateStoragePath(t *testing.T) {
	r := New(newMockStorage(), nil)
	assert.Equal(t, "helm/charts/nginx-1.0.0.tgz", r.GenerateStoragePath("nginx", "1.0.0"))
}

func TestDeprecatedMethods(t *testing.T) {
	r := New(newMockStorage(), nil)
	_, _, err := r.Download("n", "v")
	assert.Error(t, err)
	_, err = r.List(&types.ArtifactFilter{})
	assert.Error(t, err)
	assert.Error(t, r.Delete("n", "v"))
}
