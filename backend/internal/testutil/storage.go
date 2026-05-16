package testutil

import (
	"testing"
)

// VerifyFileInS3 checks if file exists in S3 and returns its content
func VerifyFileInS3(t *testing.T, env *TestEnvironment, s3Key string) []byte {
	t.Helper()

	content, err := env.Storage.Download(env.Ctx, s3Key)
	if err != nil {
		t.Fatalf("failed to download file from S3: %v", err)
	}

	return content
}

// MinioCredentials returns the endpoint and static admin credentials for the
// running MinIO container. Exposed so storage-level tests can construct
// additional S3 clients (e.g. to exercise NewS3Storage error paths) without
// reaching into testutil's internals.
func MinioCredentials(t *testing.T, env *TestEnvironment) (endpoint, accessKey, secretKey string) {
	t.Helper()

	endpoint, err := env.MinioContainer.ConnectionString(env.Ctx)
	if err != nil {
		t.Fatalf("failed to get minio endpoint: %v", err)
	}
	return endpoint, "minioadmin", "minioadmin"
}
