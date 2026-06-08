package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

const testBucket = "test-bucket"

type mockS3 struct{ err error }

func (m *mockS3) PutObject(_ context.Context, _ *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return &s3.PutObjectOutput{}, m.err
}

func (m *mockS3) DeleteObject(_ context.Context, _ *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return &s3.DeleteObjectOutput{}, m.err
}

func (m *mockS3) CreateBucket(_ context.Context, _ *s3.CreateBucketInput, _ ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	return &s3.CreateBucketOutput{}, m.err
}

func (m *mockS3) HeadBucket(_ context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return &s3.HeadBucketOutput{}, m.err
}

func TestNewS3Client_ReturnsClient(t *testing.T) {
	cfg := &config.Config{
		S3Endpoint:     "http://localhost:9000",
		S3Bucket:       testBucket,
		S3AccessKey:    "minioadmin",
		S3SecretKey:    "minioadmin",
		S3Region:       "us-east-1",
		S3UsePathStyle: true,
	}
	client, err := NewS3Client(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestUpload_Success(t *testing.T) {
	const minioEndpoint = "http://minio:9000"
	c := &S3Client{
		client:   &mockS3{err: nil},
		bucket:   testBucket,
		endpoint: minioEndpoint,
	}
	url, err := c.Upload(context.Background(), "proof/abc.jpg", strings.NewReader("data"), 4, "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, minioEndpoint+"/"+testBucket+"/proof/abc.jpg", url)
}

func TestUpload_S3Error_WrapsInternalFailure(t *testing.T) {
	c := &S3Client{
		client:   &mockS3{err: errors.New("connection refused")},
		bucket:   testBucket,
		endpoint: "http://minio:9000",
	}
	_, err := c.Upload(context.Background(), "proof/abc.jpg", strings.NewReader("data"), 4, "image/jpeg")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInternalFailure))
}
