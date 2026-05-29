package storage_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// freshExternalID returns a random external ID so chunk paths from different
// test cases never collide in the shared MinIO bucket.
func freshExternalID(prefix string) string {
	return prefix + "-" + uuid.New().String()
}

// TestUploadAndDownloadChunkRoundTrip verifies the happy-path round trip:
// UploadChunk writes the chunk, Download reads it back byte-for-byte.
func TestUploadAndDownloadChunkRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	externalID := freshExternalID("ext")
	payload := []byte("{\"line\":1}\n{\"line\":2}\n")

	key, err := env.Storage.UploadChunk(ctx, 42, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 2, payload)
	if err != nil {
		t.Fatalf("UploadChunk: %v", err)
	}
	if !strings.HasSuffix(key, "chunk_00000001_00000002.jsonl") {
		t.Errorf("unexpected key suffix: %q", key)
	}

	got, err := env.Storage.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("round-trip mismatch: got %q, want %q", got, payload)
	}
}

// TestDownloadMissingKey verifies ErrObjectNotFound classification.
func TestDownloadMissingKey(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	_, err := env.Storage.Download(context.Background(), "does/not/exist.jsonl")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !errors.Is(err, storage.ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

// TestListChunksReturnsSortedKeys verifies that ListChunks returns chunks in
// line-number order (which equals lexicographic order due to %08d padding).
func TestListChunksReturnsSortedKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	externalID := freshExternalID("list")

	// Upload three chunks in arbitrary order.
	uploads := []struct {
		first, last int
	}{
		{first: 21, last: 30},
		{first: 1, last: 10},
		{first: 11, last: 20},
	}
	for _, u := range uploads {
		if _, err := env.Storage.UploadChunk(ctx, 1, models.ProviderClaudeCode, externalID, "transcript.jsonl", u.first, u.last, []byte("x")); err != nil {
			t.Fatalf("UploadChunk(%d,%d): %v", u.first, u.last, err)
		}
	}

	keys, err := env.Storage.ListChunks(ctx, 1, models.ProviderClaudeCode, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("ListChunks: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	// Lexicographic ordering of %08d-padded chunk names equals numeric ordering.
	for i := 0; i < len(keys)-1; i++ {
		if keys[i] >= keys[i+1] {
			t.Errorf("keys out of order: %q >= %q", keys[i], keys[i+1])
		}
	}
}

func TestListChunksEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	keys, err := env.Storage.ListChunks(context.Background(), 1, models.ProviderClaudeCode, freshExternalID("empty"), "transcript.jsonl")
	if err != nil {
		t.Fatalf("ListChunks: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for nonexistent session, got %d", len(keys))
	}
}

func TestDeleteRemovesObject(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	externalID := freshExternalID("delete")

	key, err := env.Storage.UploadChunk(ctx, 1, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 5, []byte("hello"))
	if err != nil {
		t.Fatalf("UploadChunk: %v", err)
	}

	// Sanity: object exists before delete.
	if _, err := env.Storage.Download(ctx, key); err != nil {
		t.Fatalf("pre-delete Download: %v", err)
	}

	if err := env.Storage.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = env.Storage.Download(ctx, key)
	if !errors.Is(err, storage.ErrObjectNotFound) {
		t.Errorf("after delete, Download should return ErrObjectNotFound, got %v", err)
	}
}

func TestDeleteAllSessionChunksRemovesEverythingScoped(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	externalID := freshExternalID("session-delete")
	otherExternalID := freshExternalID("session-keep")

	// Two chunks under the target session
	if _, err := env.Storage.UploadChunk(ctx, 7, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 5, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Storage.UploadChunk(ctx, 7, models.ProviderClaudeCode, externalID, "agent.jsonl", 1, 5, []byte("b")); err != nil {
		t.Fatal(err)
	}
	// One under a *different* session that must survive.
	survivorKey, err := env.Storage.UploadChunk(ctx, 7, models.ProviderClaudeCode, otherExternalID, "transcript.jsonl", 1, 5, []byte("c"))
	if err != nil {
		t.Fatal(err)
	}

	if err := env.Storage.DeleteAllSessionChunks(ctx, 7, models.ProviderClaudeCode, externalID); err != nil {
		t.Fatalf("DeleteAllSessionChunks: %v", err)
	}

	// Target session should be empty.
	keys, err := env.Storage.ListChunks(ctx, 7, models.ProviderClaudeCode, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("post-delete ListChunks: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 chunks after delete, got %d", len(keys))
	}

	// Survivor must still exist.
	if _, err := env.Storage.Download(ctx, survivorKey); err != nil {
		t.Errorf("survivor chunk should still exist: %v", err)
	}
}

func TestDeleteAllUserDataRemovesEverythingForUser(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	const targetUser int64 = 555
	const otherUser int64 = 556

	if _, err := env.Storage.UploadChunk(ctx, targetUser, models.ProviderClaudeCode, freshExternalID("u-a"), "transcript.jsonl", 1, 5, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Storage.UploadChunk(ctx, targetUser, models.ProviderClaudeCode, freshExternalID("u-b"), "transcript.jsonl", 1, 5, []byte("b")); err != nil {
		t.Fatal(err)
	}
	survivorKey, err := env.Storage.UploadChunk(ctx, otherUser, models.ProviderClaudeCode, freshExternalID("u-keep"), "transcript.jsonl", 1, 5, []byte("c"))
	if err != nil {
		t.Fatal(err)
	}

	if err := env.Storage.DeleteAllUserData(ctx, targetUser); err != nil {
		t.Fatalf("DeleteAllUserData: %v", err)
	}

	if _, err := env.Storage.Download(ctx, survivorKey); err != nil {
		t.Errorf("other user's data must survive: %v", err)
	}
}

// TestDeleteAllSessionChunks_RemovesOnlyTargetProvider locks the provider-scoping
// promise in the DeleteAllSessionChunks docstring (s3.go): for the same
// (userID, externalID), chunks written under a *different* provider are NOT touched.
func TestDeleteAllSessionChunks_RemovesOnlyTargetProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	const userID int64 = 99
	externalID := freshExternalID("provider-scope")

	// Same (userID, externalID): one chunk under each provider.
	if _, err := env.Storage.UploadChunk(ctx, userID, models.ProviderClaudeCode, externalID, "transcript.jsonl", 1, 5, []byte("cc")); err != nil {
		t.Fatalf("UploadChunk(claude-code): %v", err)
	}
	if _, err := env.Storage.UploadChunk(ctx, userID, models.ProviderCodex, externalID, "transcript.jsonl", 1, 5, []byte("cx")); err != nil {
		t.Fatalf("UploadChunk(codex): %v", err)
	}

	if err := env.Storage.DeleteAllSessionChunks(ctx, userID, models.ProviderClaudeCode, externalID); err != nil {
		t.Fatalf("DeleteAllSessionChunks: %v", err)
	}

	// Target provider's chunks must be gone.
	ccKeys, err := env.Storage.ListChunks(ctx, userID, models.ProviderClaudeCode, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("post-delete ListChunks(claude-code): %v", err)
	}
	if len(ccKeys) != 0 {
		t.Errorf("target provider should have 0 chunks after delete, got %d", len(ccKeys))
	}

	// The other provider's chunk must survive.
	cxKeys, err := env.Storage.ListChunks(ctx, userID, models.ProviderCodex, externalID, "transcript.jsonl")
	if err != nil {
		t.Fatalf("post-delete ListChunks(codex): %v", err)
	}
	if len(cxKeys) != 1 {
		t.Errorf("other provider's chunk must survive: want 1, got %d", len(cxKeys))
	}
}

// TestDeleteAllSessionChunks_Empty verifies that deleting over a prefix with no
// objects is a clean no-op: ListObjects yields nothing, so the function returns nil.
func TestDeleteAllSessionChunks_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	if err := env.Storage.DeleteAllSessionChunks(context.Background(), 1, models.ProviderClaudeCode, freshExternalID("empty-session")); err != nil {
		t.Errorf("DeleteAllSessionChunks over empty prefix should return nil, got %v", err)
	}
}

// TestDeleteAllUserData_RemovesOnlyTargetUser locks the user-scoping boundary of
// DeleteAllUserData: the {userID}/ prefix must not match a string-superset user ID
// (deleting user 555 must not touch user 5550, whose decimal ID has 555 as a prefix).
// It also asserts the target's own object is actually removed.
func TestDeleteAllUserData_RemovesOnlyTargetUser(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	const targetUser int64 = 555
	const adjacentUser int64 = 5550 // decimal ID has targetUser ("555") as a prefix

	targetKey, err := env.Storage.UploadChunk(ctx, targetUser, models.ProviderClaudeCode, freshExternalID("target"), "transcript.jsonl", 1, 5, []byte("t"))
	if err != nil {
		t.Fatalf("UploadChunk(target): %v", err)
	}
	adjacentKey, err := env.Storage.UploadChunk(ctx, adjacentUser, models.ProviderClaudeCode, freshExternalID("adjacent"), "transcript.jsonl", 1, 5, []byte("a"))
	if err != nil {
		t.Fatalf("UploadChunk(adjacent): %v", err)
	}

	if err := env.Storage.DeleteAllUserData(ctx, targetUser); err != nil {
		t.Fatalf("DeleteAllUserData: %v", err)
	}

	// Target user's object must be gone.
	if _, err := env.Storage.Download(ctx, targetKey); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Errorf("target user's object should be deleted, Download returned %v", err)
	}

	// The string-superset user's object must survive the {userID}/ prefix scope.
	if _, err := env.Storage.Download(ctx, adjacentKey); err != nil {
		t.Errorf("adjacent user (5550) object must survive delete of user 555: %v", err)
	}
}

// TestDeleteAllUserData_Empty verifies that wiping a user with no objects is a
// clean no-op (returns nil).
func TestDeleteAllUserData_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	if err := env.Storage.DeleteAllUserData(context.Background(), 123456); err != nil {
		t.Errorf("DeleteAllUserData over empty prefix should return nil, got %v", err)
	}
}

func TestNewS3StorageMissingBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	// Reuse the running MinIO container's credentials, but point at a bucket
	// that definitely doesn't exist.
	endpoint, accessKey, secretKey := testutil.MinioCredentials(t, env)

	_, err := storage.NewS3Storage(storage.S3Config{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		BucketName:      "this-bucket-does-not-exist-" + uuid.New().String(),
		UseSSL:          false,
	})
	if err == nil {
		t.Fatal("expected error when bucket does not exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention missing bucket, got: %v", err)
	}
}
