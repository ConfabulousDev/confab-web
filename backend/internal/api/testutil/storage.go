package testutil

import (
	"testing"
)

// UploadTestFile uploads a file to S3 for testing
func UploadTestFile(t *testing.T, env *TestEnvironment, userID int64, sessionID, filePath string, content []byte) string {
	t.Helper()

	s3Key, err := env.Storage.Upload(env.Ctx, userID, sessionID, filePath, content)
	if err != nil {
		t.Fatalf("failed to upload test file: %v", err)
	}

	return s3Key
}

// VerifyFileInS3 checks if file exists in S3 and returns its content
func VerifyFileInS3(t *testing.T, env *TestEnvironment, s3Key string) []byte {
	t.Helper()

	content, err := env.Storage.Download(env.Ctx, s3Key)
	if err != nil {
		t.Fatalf("failed to download file from S3: %v", err)
	}

	return content
}

// AssertFileContent verifies file content matches expected
func AssertFileContent(t *testing.T, env *TestEnvironment, s3Key string, expectedContent []byte) {
	t.Helper()

	actualContent := VerifyFileInS3(t, env, s3Key)

	if string(actualContent) != string(expectedContent) {
		t.Errorf("file content mismatch.\nExpected: %s\nGot: %s", string(expectedContent), string(actualContent))
	}
}
