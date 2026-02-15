package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("confab/storage")

// Sentinel errors for storage operations
var (
	// ErrObjectNotFound indicates the requested object does not exist
	ErrObjectNotFound = errors.New("object not found")

	// ErrAccessDenied indicates insufficient permissions for the operation
	ErrAccessDenied = errors.New("access denied")

	// ErrNetworkError indicates a network connectivity issue
	ErrNetworkError = errors.New("network error")

	// ErrTooManyChunks indicates a file has exceeded the maximum allowed chunks
	ErrTooManyChunks = errors.New("file has too many chunks")
)

// MaxChunksPerFile is the maximum number of chunks allowed per file.
// This is a sanity limit to prevent unbounded memory usage when listing chunks.
// At 100 lines per chunk, this allows for 3 million lines per file.
const MaxChunksPerFile = 30000

// S3Config holds S3/MinIO configuration
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
}

// S3Storage handles object storage operations
type S3Storage struct {
	client *minio.Client
	bucket string
}

// NewS3Storage creates a new S3/MinIO storage client
func NewS3Storage(config S3Config) (*S3Storage, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Verify bucket exists (bucket must be created out-of-band)
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %q does not exist: create it before starting the server", config.BucketName)
	}

	return &S3Storage{
		client: client,
		bucket: config.BucketName,
	}, nil
}

// Download retrieves a file from S3/MinIO
func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "storage.download",
		trace.WithAttributes(attribute.String("storage.key", key)))
	defer span.End()

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, classifyStorageError(err, "download")
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, classifyStorageError(err, "download")
	}

	span.SetAttributes(attribute.Int("file.size", len(data)))
	return data, nil
}

// Delete removes a file from S3/MinIO
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "storage.delete",
		trace.WithAttributes(attribute.String("storage.key", key)))
	defer span.End()

	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete from S3: %w", err)
	}
	return nil
}

// classifyStorageError examines a storage error and returns an appropriate sentinel error
func classifyStorageError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Check for MinIO error response
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		switch minioErr.Code {
		case "NoSuchKey", "NoSuchBucket":
			return fmt.Errorf("%s: %w", operation, ErrObjectNotFound)
		case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return fmt.Errorf("%s: %w", operation, ErrAccessDenied)
		}
	}

	// Check for network/connection errors
	errStr := err.Error()
	if containsAny(errStr, []string{"connection", "timeout", "network", "dial", "refused"}) {
		return fmt.Errorf("%s network issue: %w", operation, ErrNetworkError)
	}

	// Return wrapped generic error for unknown cases
	return fmt.Errorf("%s failed: %w", operation, err)
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// ============================================================================
// Incremental Sync - Chunk Operations
// ============================================================================

// UploadChunk uploads a chunk file for incremental sync
// Key format: {user_id}/claude-code/{external_id}/chunks/{file_name}/chunk_{first:08d}_{last:08d}.jsonl
func (s *S3Storage) UploadChunk(ctx context.Context, userID int64, externalID, fileName string, firstLine, lastLine int, data []byte) (string, error) {
	ctx, span := tracer.Start(ctx, "storage.upload_chunk",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.external_id", externalID),
			attribute.String("file.name", fileName),
			attribute.Int("chunk.first_line", firstLine),
			attribute.Int("chunk.last_line", lastLine),
			attribute.Int("file.size", len(data)),
		))
	defer span.End()

	key := fmt.Sprintf("%d/claude-code/%s/chunks/%s/chunk_%08d_%08d.jsonl",
		userID, externalID, fileName, firstLine, lastLine)

	reader := bytes.NewReader(data)
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", classifyStorageError(err, "upload chunk")
	}

	return key, nil
}

// ListChunks lists all chunk files for a given session and file name
// Returns keys sorted by name (which gives correct line order due to zero-padded naming)
// Returns ErrTooManyChunks if the file exceeds MaxChunksPerFile.
func (s *S3Storage) ListChunks(ctx context.Context, userID int64, externalID, fileName string) ([]string, error) {
	ctx, span := tracer.Start(ctx, "storage.list_chunks",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.external_id", externalID),
			attribute.String("file.name", fileName),
		))
	defer span.End()

	prefix := fmt.Sprintf("%d/claude-code/%s/chunks/%s/", userID, externalID, fileName)

	var keys []string
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			span.RecordError(obj.Err)
			span.SetStatus(codes.Error, obj.Err.Error())
			return nil, classifyStorageError(obj.Err, "list chunks")
		}
		keys = append(keys, obj.Key)

		// Sanity check to prevent unbounded memory usage
		if len(keys) > MaxChunksPerFile {
			err := fmt.Errorf("list chunks: %w (limit: %d)", ErrTooManyChunks, MaxChunksPerFile)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	}

	span.SetAttributes(attribute.Int("chunks.count", len(keys)))

	// Keys are already sorted by ListObjects (lexicographic order)
	// Due to zero-padded line numbers, this gives correct order
	return keys, nil
}

// DeleteAllSessionChunks deletes all chunks for all files in a session
func (s *S3Storage) DeleteAllSessionChunks(ctx context.Context, userID int64, externalID string) error {
	ctx, span := tracer.Start(ctx, "storage.delete_all_session_chunks",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.external_id", externalID),
		))
	defer span.End()

	prefix := fmt.Sprintf("%d/claude-code/%s/chunks/", userID, externalID)

	var deletedCount int
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			span.RecordError(obj.Err)
			span.SetStatus(codes.Error, obj.Err.Error())
			return classifyStorageError(obj.Err, "list session chunks")
		}
		if err := s.Delete(ctx, obj.Key); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("failed to delete chunk %s: %w", obj.Key, err)
		}
		deletedCount++
	}

	span.SetAttributes(attribute.Int("chunks.deleted", deletedCount))

	return nil
}
