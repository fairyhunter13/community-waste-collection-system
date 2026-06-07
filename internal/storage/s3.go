// Package storage provides S3-compatible file storage for proof of payment uploads.
package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// s3API is the subset of aws-sdk-go-v2 s3.Client used by S3Client, enabling test injection.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
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
		awsconfig.WithHTTPClient(&http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}),
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

// EnsureBucket creates the configured bucket if it does not already exist.
// Call this once at application startup.
func (c *S3Client) EnsureBucket(ctx context.Context) error {
	ctx, span := observability.Tracer().Start(ctx, "storage.s3.EnsureBucket")
	defer span.End()
	span.SetAttributes(attribute.String("s3.bucket", c.bucket))

	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	if err == nil {
		return nil
	}
	_, err = c.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.bucket)})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		observability.FromContext(ctx).ErrorContext(ctx, "ensure S3 bucket failed",
			slog.String("op", "S3.EnsureBucket"),
			slog.String("bucket", c.bucket),
			slog.Any("err", err),
		)
		return fmt.Errorf("create bucket %s: %w", c.bucket, err)
	}
	return nil
}

// Upload uploads a file to S3-compatible storage and returns the object's public URL.
func (c *S3Client) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	ctx, span := observability.Tracer().Start(ctx, "storage.s3.Upload")
	defer span.End()
	span.SetAttributes(
		attribute.String("s3.bucket", c.bucket),
		attribute.String("s3.key", key),
		attribute.String("s3.content_type", contentType),
		attribute.Int64("s3.size_bytes", size),
	)

	start := time.Now()
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	observability.S3UploadDurationSeconds.Observe(time.Since(start).Seconds())

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		observability.S3ErrorsTotal.Inc()
		observability.FromContext(ctx).ErrorContext(ctx, "S3 upload failed",
			slog.String("op", "S3.Upload"),
			slog.String("bucket", c.bucket),
			slog.String("key", key),
			slog.Any("err", err),
		)
		return "", fmt.Errorf("s3 upload %s: %w", key, domain.ErrInternalFailure)
	}
	observability.S3UploadBytesTotal.Add(float64(size))
	return fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key), nil
}
