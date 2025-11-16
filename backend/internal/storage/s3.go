package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

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
func (s *S3Storage) Upload(ctx context.Context, userID int64, sessionID, filename string, data []byte) (string, error) {
	// Organize files by user/session
	key := s.generateKey(userID, sessionID, filename)

	reader := bytes.NewReader(data)
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json", // All our files are JSONL
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return key, nil
}

// Download retrieves a file from S3/MinIO
func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %w", err)
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
// Format: {user_id}/{session_id}/{filename}
func (s *S3Storage) generateKey(userID int64, sessionID, filename string) string {
	basename := filepath.Base(filename)
	return fmt.Sprintf("%d/%s/%s", userID, sessionID, basename)
}
