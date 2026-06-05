// Package storage provides S3-compatible file storage for proof of payment uploads.
package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

// s3API is the subset of aws-sdk-go-v2 s3.Client used by S3Client, enabling test injection.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Client is an S3-compatible storage client backed by aws-sdk-go-v2.
type S3Client struct {
	client   s3API
	bucket   string
	endpoint string
}

// NewS3Client creates a new S3Client configured for the given application settings.
func NewS3Client(cfg *config.Config) (*S3Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey, cfg.S3SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3UsePathStyle
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
	})

	return &S3Client{
		client:   client,
		bucket:   cfg.S3Bucket,
		endpoint: cfg.S3Endpoint,
	}, nil
}

// Upload uploads a file to S3-compatible storage and returns the object's public URL.
func (c *S3Client) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload %s: %w", key, domain.ErrInternalFailure)
	}
	return fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key), nil
}
