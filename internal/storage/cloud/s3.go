package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/lgulliver/lodestone/pkg/config"
)

// S3Storage implements BlobStorage for S3-compatible object storage.
type S3Storage struct {
	client *s3.Client
	bucket string
}

// NewS3Storage creates a new S3 storage backend from storage configuration.
func NewS3Storage(storageCfg *config.StorageConfig) (*S3Storage, error) {
	s3Cfg := storageCfg.S3
	if s3Cfg.Bucket == "" {
		return nil, fmt.Errorf("missing required s3 configuration: bucket")
	}
	if s3Cfg.Region == "" {
		return nil, fmt.Errorf("missing required s3 configuration: region")
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s3Cfg.Region),
	}

	if s3Cfg.AccessKey != "" && s3Cfg.SecretKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s3Cfg.AccessKey, s3Cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load s3 aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if s3Cfg.Endpoint != "" {
			o.BaseEndpoint = &s3Cfg.Endpoint
		}
		o.UsePathStyle = s3Cfg.ForcePathStyle
	})

	return &S3Storage{
		client: client,
		bucket: s3Cfg.Bucket,
	}, nil
}

func (s *S3Storage) Store(ctx context.Context, path string, content io.Reader, contentType string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
		Body:   content,
	}
	if contentType != "" {
		input.ContentType = &contentType
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("failed to store object %q in s3: %w", path, err)
	}
	return nil
}

func (s *S3Storage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to retrieve object %q from s3: %w", path, err)
	}
	return output.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	}); err != nil {
		return fmt.Errorf("failed to delete object %q from s3: %w", path, err)
	}
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	})
	if err == nil {
		return true, nil
	}
	if isS3NotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check object %q in s3: %w", path, err)
}

func (s *S3Storage) GetSize(ctx context.Context, path string) (int64, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	})
	if err != nil {
		if isS3NotFound(err) {
			return 0, fmt.Errorf("file not found: %s", path)
		}
		return 0, fmt.Errorf("failed to get object size %q from s3: %w", path, err)
	}
	if output.ContentLength == nil {
		return 0, nil
	}
	return *output.ContentLength, nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in s3 with prefix %q: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}
	return keys, nil
}

func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return strings.EqualFold(code, "NotFound") ||
			strings.EqualFold(code, "NoSuchKey") ||
			strings.EqualFold(code, "NoSuchBucket")
	}
	return false
}
