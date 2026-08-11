package videos

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// VideoStorage is the only thing this domain's business logic knows about
// object storage. Today's implementation talks to MinIO, but nothing above
// this interface (Service, Handler) depends on that — swapping in real AWS
// S3 or Cloudflare R2 later means writing one new VideoStorage and changing
// the constructor call in main.go, nothing else.
type VideoStorage interface {
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	DeleteObject(ctx context.Context, key string) error
	// DeletePrefix removes every object whose key starts with prefix — used
	// to clean up an entire superseded video's HLS tree (many segment
	// files) in one call rather than tracking each key individually.
	DeletePrefix(ctx context.Context, prefix string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	ObjectExists(ctx context.Context, key string) (bool, error)
	// GetObject streams an object's body — used by the backend-authorized
	// HLS proxy (playlists/segments) and by the worker (downloading the
	// source to transcode), never by anything that would buffer a whole
	// video in memory.
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
	// Ping is used only for a best-effort startup health check — see
	// main.go — never on the request path.
	Ping(ctx context.Context) error
}

// S3Storage implements VideoStorage against any S3-compatible endpoint
// (MinIO in development). It holds two clients on purpose: internalClient
// talks to the storage over the Docker-internal endpoint for Put/Delete/Head,
// while presignClient is built against the publicly-reachable endpoint so
// the URLs it signs actually resolve from a student's browser.
type S3Storage struct {
	internalClient *s3.Client
	presignClient  *s3.PresignClient
	uploader       *manager.Uploader
	bucket         string
}

type S3Config struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	Region         string
}

func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	internalClient := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // required for MinIO; virtual-hosted-style buckets don't resolve
	})

	publicClient := internalClient
	if cfg.PublicEndpoint != "" && cfg.PublicEndpoint != cfg.Endpoint {
		publicClient = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.PublicEndpoint)
			o.UsePathStyle = true
		})
	}

	return &S3Storage{
		internalClient: internalClient,
		presignClient:  s3.NewPresignClient(publicClient),
		uploader:       manager.NewUploader(internalClient),
		bucket:         cfg.Bucket,
	}, nil
}

func (s *S3Storage) PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	return err
}

func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.internalClient.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Storage) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *S3Storage) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.internalClient.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Storage) Ping(ctx context.Context) error {
	_, err := s.internalClient.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}

func (s *S3Storage) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	out, err := s.internalClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, "", err
	}
	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	size := int64(0)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, size, contentType, nil
}

// DeletePrefix lists (paginated) and batch-deletes every object under
// prefix. Best-effort by design — see Service.activateAndCleanup — a
// partial failure here leaves orphaned objects in MinIO, which is an
// accepted, non-corrupting outcome (cleanup can be retried or swept later)
// rather than something that should ever roll back an already-committed
// activation.
func (s *S3Storage) DeletePrefix(ctx context.Context, prefix string) error {
	paginator := s3.NewListObjectsV2Paginator(s.internalClient, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			continue
		}

		ids := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			ids = append(ids, types.ObjectIdentifier{Key: obj.Key})
		}

		if _, err := s.internalClient.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: ids},
		}); err != nil {
			return err
		}
	}
	return nil
}
