package registry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/internal/registry/upstream"
	"github.com/lgulliver/lodestone/pkg/types"
)

func TestUpload_RegistryDisabled(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	require.NoError(t, service.Settings.DisableRegistry(ctx, "test", user.ID))

	mockHandler := &MockHandler{}
	service.handlers["test"] = mockHandler

	artifact, err := service.Upload(ctx, "test", "pkg", "1.0.0", bytes.NewReader([]byte("x")), user.ID)
	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "disabled")
}

func TestUpload_MetadataFailure(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	mockHandler := &MockHandler{}
	service.handlers["test"] = mockHandler

	content := []byte("c")
	mockHandler.On("Validate", mock.AnythingOfType("*types.Artifact"), content).Return(nil)
	mockHandler.On("GetMetadata", content).Return(map[string]interface{}{}, errors.New("boom"))

	artifact, err := service.Upload(ctx, "test", "pkg", "1.0.0", bytes.NewReader(content), user.ID)
	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "extract metadata")
	mockHandler.AssertExpectations(t)
}

func TestUpload_HandlerUploadFailure(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	mockHandler := &MockHandler{}
	service.handlers["test"] = mockHandler

	content := []byte("c")
	mockHandler.On("Validate", mock.AnythingOfType("*types.Artifact"), content).Return(nil)
	mockHandler.On("GetMetadata", content).Return(map[string]interface{}{}, nil)
	mockHandler.On("GenerateStoragePath", "pkg", "1.0.0").Return("test/pkg/1.0.0/artifact")
	mockHandler.On("Upload", ctx, mock.AnythingOfType("*types.Artifact"), content).Return(errors.New("store fail"))

	artifact, err := service.Upload(ctx, "test", "pkg", "1.0.0", bytes.NewReader(content), user.ID)
	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "failed to upload artifact")
	mockHandler.AssertExpectations(t)
}

func TestDownload_RegistryDisabled(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()

	require.NoError(t, service.Settings.DisableRegistry(ctx, "test", user.ID))
	service.handlers["test"] = &MockHandler{}

	_, _, err := service.Download(ctx, "test", "pkg", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestDownload_StorageRetrieveFailure(t *testing.T) {
	service, db, mockStorage := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()
	service.handlers["test"] = &MockHandler{}

	art := &types.Artifact{
		Name: "pkg", Version: "1.0.0", Registry: "test",
		StoragePath: "test/pkg/1.0.0/artifact", PublishedBy: user.ID,
	}
	require.NoError(t, db.Create(art).Error)

	mockStorage.On("Retrieve", ctx, "test/pkg/1.0.0/artifact").
		Return(io.NopCloser(bytes.NewReader(nil)), errors.New("gone"))

	_, _, err := service.Download(ctx, "test", "pkg", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve artifact")
	mockStorage.AssertExpectations(t)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read fail") }

func TestUpload_ContentReadFailure(t *testing.T) {
	service, db, _ := setupTestService(t)
	user := createTestUser(t, db)
	ctx := context.Background()
	service.handlers["test"] = &MockHandler{}

	artifact, err := service.Upload(ctx, "test", "pkg", "1.0.0", errReader{}, user.ID)
	assert.Error(t, err)
	assert.Nil(t, artifact)
	assert.Contains(t, err.Error(), "failed to read content")
}

func TestEnsureUpstreamCached_ProxyDisabled(t *testing.T) {
	service, _, _ := setupTestService(t)
	err := service.EnsureUpstreamCached(context.Background(), "npm", "pkg", "1.0.0", "artifact")
	assert.ErrorIs(t, err, upstream.ErrProxyDisabled)
}
