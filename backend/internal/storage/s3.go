package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

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

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, config.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &S3Storage{
		client: client,
		bucket: config.BucketName,
	}, nil
}

// Upload uploads a file to S3/MinIO
// Returns the S3 key where the file was stored
func (s *S3Storage) Upload(ctx context.Context, userID int64, sessionType, externalID string, runID int64, filename string, data []byte) (string, error) {
	// Organize files by user/session_type/external_id/run_id
	key := s.generateKey(userID, sessionType, externalID, runID, filename)

	reader := bytes.NewReader(data)
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json", // All our files are JSONL
	})
	if err != nil {
		return "", classifyStorageError(err, "upload")
	}

	return key, nil
}

// Download retrieves a file from S3/MinIO
func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, classifyStorageError(err, "download")
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		// Check if error is from S3 response (e.g., NoSuchKey)
		return nil, classifyStorageError(err, "download")
	}

	return data, nil
}

// Delete removes a file from S3/MinIO
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}
	return nil
}

// generateKey creates an organized S3 key path
// Format: {user_id}/{session_type}/{external_id}/{run_id}/{filename}
func (s *S3Storage) generateKey(userID int64, sessionType, externalID string, runID int64, filename string) string {
	basename := filepath.Base(filename)
	// Normalize session type: lowercase, spaces to hyphens
	normalizedType := normalizeSessionType(sessionType)
	return fmt.Sprintf("%d/%s/%s/%d/%s", userID, normalizedType, externalID, runID, basename)
}

// normalizeSessionType converts session type to a safe S3 key component
// e.g., "Claude Code" -> "claude-code"
func normalizeSessionType(sessionType string) string {
	result := make([]byte, 0, len(sessionType))
	for i := 0; i < len(sessionType); i++ {
		c := sessionType[i]
		if c >= 'A' && c <= 'Z' {
			result = append(result, c+32) // lowercase
		} else if c == ' ' {
			result = append(result, '-')
		} else if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
		// skip other characters
	}
	return string(result)
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
	key := fmt.Sprintf("%d/claude-code/%s/chunks/%s/chunk_%08d_%08d.jsonl",
		userID, externalID, fileName, firstLine, lastLine)

	reader := bytes.NewReader(data)
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return "", classifyStorageError(err, "upload chunk")
	}

	return key, nil
}

// ListChunks lists all chunk files for a given session and file name
// Returns keys sorted by name (which gives correct line order due to zero-padded naming)
// Returns ErrTooManyChunks if the file exceeds MaxChunksPerFile.
func (s *S3Storage) ListChunks(ctx context.Context, userID int64, externalID, fileName string) ([]string, error) {
	prefix := fmt.Sprintf("%d/claude-code/%s/chunks/%s/", userID, externalID, fileName)

	var keys []string
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return nil, classifyStorageError(obj.Err, "list chunks")
		}
		keys = append(keys, obj.Key)

		// Sanity check to prevent unbounded memory usage
		if len(keys) > MaxChunksPerFile {
			return nil, fmt.Errorf("list chunks: %w (limit: %d)", ErrTooManyChunks, MaxChunksPerFile)
		}
	}

	// Keys are already sorted by ListObjects (lexicographic order)
	// Due to zero-padded line numbers, this gives correct order
	return keys, nil
}

// DeleteChunks deletes all chunks for a session/file
func (s *S3Storage) DeleteChunks(ctx context.Context, userID int64, externalID, fileName string) error {
	keys, err := s.ListChunks(ctx, userID, externalID, fileName)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return fmt.Errorf("failed to delete chunk %s: %w", key, err)
		}
	}

	return nil
}

// DeleteAllSessionChunks deletes all chunks for all files in a session
func (s *S3Storage) DeleteAllSessionChunks(ctx context.Context, userID int64, externalID string) error {
	prefix := fmt.Sprintf("%d/claude-code/%s/chunks/", userID, externalID)

	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return classifyStorageError(obj.Err, "list session chunks")
		}
		if err := s.Delete(ctx, obj.Key); err != nil {
			return fmt.Errorf("failed to delete chunk %s: %w", obj.Key, err)
		}
	}

	return nil
}
