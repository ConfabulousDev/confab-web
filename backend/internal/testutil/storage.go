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
