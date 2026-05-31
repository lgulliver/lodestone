package oci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/types"
)

// mockStorage is an in-memory BlobStorage. Failure flags exercise error paths.
type mockStorage struct {
	data       map[string][]byte
	failStore  bool
	failExists bool
	failSize   bool
	failRetr   bool
	failList   bool
}

func newMockStorage() *mockStorage { return &mockStorage{data: make(map[string][]byte)} }

func (m *mockStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	if m.failStore {
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
	if m.failRetr {
		return nil, fmt.Errorf("retrieve failed")
	}
	if data, ok := m.data[path]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockStorage) Delete(ctx context.Context, path string) error { delete(m.data, path); return nil }
func (m *mockStorage) Exists(ctx context.Context, path string) (bool, error) {
	if m.failExists {
		return false, fmt.Errorf("exists failed")
	}
	_, ok := m.data[path]
	return ok, nil
}
func (m *mockStorage) GetSize(ctx context.Context, path string) (int64, error) {
	if m.failSize {
		return 0, fmt.Errorf("size failed")
	}
	if data, ok := m.data[path]; ok {
		return int64(len(data)), nil
	}
	return 0, fmt.Errorf("not found")
}
func (m *mockStorage) List(ctx context.Context, prefix string) ([]string, error) {
	if m.failList {
		return nil, fmt.Errorf("list failed")
	}
	var out []string
	for p := range m.data {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out, nil
}

func digestOf(b []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}

func TestValidate(t *testing.T) {
	r := New(newMockStorage(), nil)
	tests := []struct {
		name    string
		art     *types.Artifact
		content []byte
		wantErr string
	}{
		{"valid tag", &types.Artifact{Name: "library/nginx", Version: "latest"}, []byte("x"), ""},
		{"valid digest", &types.Artifact{Name: "app", Version: "sha256:" + strings.Repeat("a", 64)}, []byte("x"), ""},
		{"empty content", &types.Artifact{Name: "app", Version: "latest"}, nil, "empty blob content"},
		{"invalid image name", &types.Artifact{Name: "BadName", Version: "latest"}, []byte("x"), "invalid OCI image name format"},
		{"invalid digest", &types.Artifact{Name: "app", Version: "sha256:zzz"}, []byte("x"), "invalid digest format"},
		{"invalid tag", &types.Artifact{Name: "app", Version: "-bad"}, []byte("x"), "invalid tag format"},
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
	md, err := New(newMockStorage(), nil).GetMetadata([]byte("abc"))
	require.NoError(t, err)
	assert.Equal(t, "oci", md["format"])
	assert.Equal(t, 3, md["size"])
}

func TestGenerateStoragePath(t *testing.T) {
	r := New(newMockStorage(), nil)
	assert.Equal(t, "oci/app/manifests/latest", r.GenerateStoragePath("app", "latest"))
	assert.Equal(t, "oci/app/blobs/sha256/abc", r.GenerateStoragePath("app", "sha256:abc"))
}

func TestUploadAndDeprecated(t *testing.T) {
	st := newMockStorage()
	r := New(st, nil)
	art := &types.Artifact{StoragePath: "oci/app/blobs/x"}
	require.NoError(t, r.Upload(context.Background(), art, []byte("blob")))
	assert.Equal(t, "application/octet-stream", art.ContentType)

	st.failStore = true
	require.Error(t, r.Upload(context.Background(), &types.Artifact{StoragePath: "p"}, []byte("x")))

	_, _, err := r.Download("n", "v")
	assert.Error(t, err)
	_, err = r.List(&types.ArtifactFilter{})
	assert.Error(t, err)
	assert.Error(t, r.Delete("n", "v"))
}

func TestBlobLifecycle(t *testing.T) {
	st := newMockStorage()
	r := New(st, nil)
	ctx := context.Background()
	repo, digest := "app", "sha256:deadbeef"
	path := fmt.Sprintf("oci/%s/blobs/%s", repo, digest)

	// Not present yet.
	exists, size, err := r.BlobExists(ctx, repo, digest)
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Zero(t, size)

	// GetBlob on missing blob errors.
	_, _, err = r.GetBlob(ctx, repo, digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blob not found")

	// Store then check.
	st.data[path] = []byte("blobdata")
	exists, size, err = r.BlobExists(ctx, repo, digest)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.EqualValues(t, 8, size)

	reader, size, err := r.GetBlob(ctx, repo, digest)
	require.NoError(t, err)
	got, _ := io.ReadAll(reader)
	assert.Equal(t, []byte("blobdata"), got)
	assert.EqualValues(t, 8, size)

	require.NoError(t, r.DeleteBlob(ctx, repo, digest))
	exists, _, _ = r.BlobExists(ctx, repo, digest)
	assert.False(t, exists)
}

func TestBlobExistsError(t *testing.T) {
	st := newMockStorage()
	st.failExists = true
	r := New(st, nil)
	_, _, err := r.GetBlob(context.Background(), "app", "sha256:x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check blob existence")
}

func TestManifestLifecycle(t *testing.T) {
	st := newMockStorage()
	r := New(st, nil)
	ctx := context.Background()
	repo, ref := "app", "v1"
	manifest := []byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json"}`)

	// Missing manifest reports not-exists without error.
	exists, _, _, _, err := r.ManifestExists(ctx, repo, ref)
	require.NoError(t, err)
	assert.False(t, exists)

	// GetManifest on missing errors.
	_, _, _, err = r.GetManifest(ctx, repo, ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest not found")

	// Put manifest.
	digest, err := r.PutManifest(ctx, repo, ref, bytes.NewReader(manifest), "application/vnd.oci.image.manifest.v1+json")
	require.NoError(t, err)
	assert.Equal(t, digestOf(manifest), digest)

	// Exists now, with media type parsed from JSON.
	exists, gotDigest, size, mediaType, err := r.ManifestExists(ctx, repo, ref)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, digestOf(manifest), gotDigest)
	assert.EqualValues(t, len(manifest), size)
	assert.Equal(t, "application/vnd.oci.image.manifest.v1+json", mediaType)

	// Get manifest content back.
	reader, gotDigest, _, err := r.GetManifest(ctx, repo, ref)
	require.NoError(t, err)
	body, _ := io.ReadAll(reader)
	assert.Equal(t, manifest, body)
	assert.Equal(t, digest, gotDigest)

	// Delete.
	require.NoError(t, r.DeleteManifest(ctx, repo, ref))
	exists, _, _, _, _ = r.ManifestExists(ctx, repo, ref)
	assert.False(t, exists)
}

func TestPutManifestInvalidJSON(t *testing.T) {
	r := New(newMockStorage(), nil)
	_, err := r.PutManifest(context.Background(), "app", "v1", bytes.NewReader([]byte("not json")),
		"application/vnd.docker.distribution.manifest.v2+json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid manifest JSON")
}

func TestListTagsAndRepositories(t *testing.T) {
	st := newMockStorage()
	r := New(st, nil)
	ctx := context.Background()
	st.data["oci/app/manifests/v1"] = []byte("m")
	st.data["oci/app/manifests/v2"] = []byte("m")
	st.data["oci/app/manifests/sha256:abc"] = []byte("m") // digest entries skipped
	st.data["oci/other/blobs/sha256:x"] = []byte("b")

	tags, err := r.ListTags(ctx, "app")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"v1", "v2"}, tags)

	repos, err := r.ListRepositories(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app", "other"}, repos)
}

func TestListErrors(t *testing.T) {
	st := newMockStorage()
	st.failList = true
	r := New(st, nil)
	_, err := r.ListTags(context.Background(), "app")
	require.Error(t, err)
	_, err = r.ListRepositories(context.Background())
	require.Error(t, err)
}

func TestSessionManagerUploadFlow(t *testing.T) {
	st := newMockStorage()
	r := New(st, nil)
	ctx := context.Background()

	sess, err := r.StartBlobUpload(ctx, "app", "user1")
	require.NoError(t, err)
	assert.NotEmpty(t, sess.ID)

	// Status before any chunk.
	status, err := r.GetBlobUploadStatus(sess.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, status.Size)

	// Two chunks → combined blob.
	_, err = r.AppendBlobChunk(ctx, sess.ID, bytes.NewReader([]byte("hello ")), "")
	require.NoError(t, err)
	updated, err := r.AppendBlobChunk(ctx, sess.ID, bytes.NewReader([]byte("world")), "")
	require.NoError(t, err)
	assert.EqualValues(t, 11, updated.Size)

	// Complete with correct digest.
	full := []byte("hello world")
	_, finalPath, err := r.CompleteBlobUpload(ctx, sess.ID, digestOf(full))
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("oci/app/blobs/%s", digestOf(full)), finalPath)
	assert.Equal(t, full, st.data[finalPath])
}

func TestSessionManagerNegativePaths(t *testing.T) {
	st := newMockStorage()
	r := New(st, nil)
	ctx := context.Background()

	// Unknown session.
	_, err := r.AppendBlobChunk(ctx, "nope", bytes.NewReader([]byte("x")), "")
	assert.Error(t, err)
	_, _, err = r.CompleteBlobUpload(ctx, "nope", "")
	assert.Error(t, err)
	_, err = r.GetBlobUploadStatus("nope")
	assert.Error(t, err)
	assert.Error(t, r.CancelBlobUpload(ctx, "nope"))

	// Digest mismatch.
	sess, _ := r.StartBlobUpload(ctx, "app", "u")
	_, _ = r.AppendBlobChunk(ctx, sess.ID, bytes.NewReader([]byte("data")), "")
	_, _, err = r.CompleteBlobUpload(ctx, sess.ID, "sha256:"+strings.Repeat("0", 64))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "digest mismatch")

	// Cancel removes the session.
	sess2, _ := r.StartBlobUpload(ctx, "app", "u")
	require.NoError(t, r.CancelBlobUpload(ctx, sess2.ID))
	_, err = r.GetBlobUploadStatus(sess2.ID)
	assert.Error(t, err)
}

func TestCleanupExpiredSessions(t *testing.T) {
	st := newMockStorage()
	sm := NewSessionManager(st)
	ctx := context.Background()
	sess, _ := sm.StartUpload(ctx, "app", "u")

	// Force expiry.
	sess.LastUpdate = sess.LastUpdate.Add(-48 * 60 * 60 * 1e9)
	sm.cleanupExpiredSessions()

	_, exists := sm.GetSession(sess.ID)
	assert.False(t, exists)
}
