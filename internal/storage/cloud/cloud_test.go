package cloud

import (
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lgulliver/lodestone/pkg/config"
)

func TestNewS3StorageValidation(t *testing.T) {
	_, err := NewS3Storage(&config.StorageConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket")

	_, err = NewS3Storage(&config.StorageConfig{S3: config.S3StorageConfig{Bucket: "b"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region")
}

func TestNewS3StorageHappyPath(t *testing.T) {
	// LoadDefaultConfig + client construction do not require network access.
	s, err := NewS3Storage(&config.StorageConfig{S3: config.S3StorageConfig{
		Bucket:         "b",
		Region:         "us-east-1",
		AccessKey:      "ak",
		SecretKey:      "sk",
		Endpoint:       "http://localhost:9000",
		ForcePathStyle: true,
	}})
	require.NoError(t, err)
	assert.Equal(t, "b", s.bucket)
}

func TestNewAzureBlobStorageValidation(t *testing.T) {
	_, err := NewAzureBlobStorage(&config.StorageConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container")

	// Container set but no creds and no connection string.
	_, err = NewAzureBlobStorage(&config.StorageConfig{Azure: config.AzureStorageConfig{Container: "c"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account name/key or connection string")
}

func TestIsS3NotFound(t *testing.T) {
	assert.True(t, isS3NotFound(&smithy.GenericAPIError{Code: "NoSuchKey"}))
	assert.True(t, isS3NotFound(&smithy.GenericAPIError{Code: "NotFound"}))
	assert.True(t, isS3NotFound(&smithy.GenericAPIError{Code: "NoSuchBucket"}))
	assert.False(t, isS3NotFound(&smithy.GenericAPIError{Code: "AccessDenied"}))
	assert.False(t, isS3NotFound(errors.New("plain error")))
}

func TestIsAzureNotFound(t *testing.T) {
	assert.True(t, isAzureNotFound(&azcore.ResponseError{StatusCode: 404}))
	assert.True(t, isAzureNotFound(&azcore.ResponseError{ErrorCode: string(bloberror.BlobNotFound)}))
	assert.False(t, isAzureNotFound(&azcore.ResponseError{StatusCode: 500}))
	assert.False(t, isAzureNotFound(errors.New("plain error")))
}

func TestIsAzureContainerExists(t *testing.T) {
	assert.True(t, isAzureContainerExists(&azcore.ResponseError{ErrorCode: string(bloberror.ContainerAlreadyExists)}))
	assert.False(t, isAzureContainerExists(&azcore.ResponseError{ErrorCode: "Other"}))
	assert.False(t, isAzureContainerExists(errors.New("plain error")))
}
