package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"

	"github.com/lgulliver/lodestone/pkg/config"
)

// AzureBlobStorage implements BlobStorage for Azure Blob Storage.
type AzureBlobStorage struct {
	client    *azblob.Client
	container string
}

// NewAzureBlobStorage creates a new Azure blob storage backend from storage configuration.
func NewAzureBlobStorage(storageCfg *config.StorageConfig) (*AzureBlobStorage, error) {
	azureCfg := storageCfg.Azure
	if azureCfg.Container == "" {
		return nil, fmt.Errorf("missing required azure configuration: container")
	}

	var (
		client *azblob.Client
		err    error
	)

	if azureCfg.ConnectionString != "" {
		client, err = azblob.NewClientFromConnectionString(azureCfg.ConnectionString, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create azure client from connection string: %w", err)
		}
	} else {
		if azureCfg.AccountName == "" || azureCfg.AccountKey == "" {
			return nil, fmt.Errorf("missing required azure configuration: account name/key or connection string")
		}
		credential, credErr := azblob.NewSharedKeyCredential(azureCfg.AccountName, azureCfg.AccountKey)
		if credErr != nil {
			return nil, fmt.Errorf("failed to create azure shared key credential: %w", credErr)
		}

		endpoint := azureCfg.Endpoint
		if endpoint == "" {
			endpoint = fmt.Sprintf("https://%s.blob.core.windows.net/", azureCfg.AccountName)
		}

		client, err = azblob.NewClientWithSharedKeyCredential(endpoint, credential, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create azure blob client: %w", err)
		}
	}

	if _, err := client.CreateContainer(context.Background(), azureCfg.Container, nil); err != nil {
		if !isAzureContainerExists(err) {
			return nil, fmt.Errorf("failed to ensure azure container %q: %w", azureCfg.Container, err)
		}
	}

	return &AzureBlobStorage{
		client:    client,
		container: azureCfg.Container,
	}, nil
}

func (a *AzureBlobStorage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	options := &azblob.UploadStreamOptions{}
	if contentType != "" {
		options.HTTPHeaders = &blob.HTTPHeaders{
			BlobContentType: &contentType,
		}
	}
	if _, err := a.client.UploadStream(ctx, a.container, path, content, options); err != nil {
		return fmt.Errorf("failed to store blob %q in azure: %w", path, err)
	}
	return nil
}

func (a *AzureBlobStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	output, err := a.client.DownloadStream(ctx, a.container, path, nil)
	if err != nil {
		if isAzureNotFound(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to retrieve blob %q from azure: %w", path, err)
	}
	return output.Body, nil
}

func (a *AzureBlobStorage) Delete(ctx context.Context, path string) error {
	_, err := a.client.DeleteBlob(ctx, a.container, path, nil)
	if err != nil && !isAzureNotFound(err) {
		return fmt.Errorf("failed to delete blob %q from azure: %w", path, err)
	}
	return nil
}

func (a *AzureBlobStorage) Exists(ctx context.Context, path string) (bool, error) {
	blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlobClient(path)
	_, err := blobClient.GetProperties(ctx, nil)
	if err == nil {
		return true, nil
	}
	if isAzureNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check blob %q in azure: %w", path, err)
}

func (a *AzureBlobStorage) GetSize(ctx context.Context, path string) (int64, error) {
	blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlobClient(path)
	output, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		if isAzureNotFound(err) {
			return 0, fmt.Errorf("file not found: %s", path)
		}
		return 0, fmt.Errorf("failed to get blob size %q in azure: %w", path, err)
	}
	if output.ContentLength == nil {
		return 0, nil
	}
	return *output.ContentLength, nil
}

func (a *AzureBlobStorage) List(ctx context.Context, prefix string) ([]string, error) {
	pager := a.client.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	var keys []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs in azure with prefix %q: %w", prefix, err)
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name != nil {
				keys = append(keys, *item.Name)
			}
		}
	}
	return keys, nil
}

func isAzureNotFound(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404 ||
			strings.EqualFold(respErr.ErrorCode, string(bloberror.BlobNotFound))
	}
	return false
}

func isAzureContainerExists(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return strings.EqualFold(respErr.ErrorCode, string(bloberror.ContainerAlreadyExists))
	}
	return false
}
