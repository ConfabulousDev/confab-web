package codex

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// rootUUID/childUUID/grandchildUUID are fixed across tests so SQL traces are
// easy to read when something fails.
const (
	rootUUID       = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	childUUID      = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	grandchildUUID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	siblingUUID    = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
)

func ptr(s string) *string { return &s }

// setupCodexEnv creates a user + codex session + the codex store, returning
// them in one bundle. The session_type is set to "codex" because in
// production all writers to codex_rollouts go through a codex session.
func setupCodexEnv(t *testing.T) (*Store, int64, string, context.Context) {
	t.Helper()
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "rollout@test.com", "Rollout User")
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "ext-codex", "codex")
	return &Store{DB: env.DB}, user.ID, sessionID, context.Background()
}

func makeParams(threadUUID, sessionID, hostedFile string, parent *string) UpsertRolloutParams {
	return UpsertRolloutParams{
		ThreadUUID:       threadUUID,
		ParentThreadUUID: parent,
		HostedSessionID:  sessionID,
		HostedFileName:   hostedFile,
		RolloutPath:      "/home/u/.codex/sessions/" + threadUUID + ".jsonl",
	}
}

func TestUpsertRollout_RootInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	if err := store.UpsertRollout(ctx, userID, makeParams(rootUUID, sessionID, "rollout-root.jsonl", nil)); err != nil {
		t.Fatalf("UpsertRollout: %v", err)
	}

	got, err := store.GetRollout(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.ThreadUUID != rootUUID {
		t.Errorf("ThreadUUID = %q, want %q", got.ThreadUUID, rootUUID)
	}
	if got.ParentThreadUUID != nil {
		t.Errorf("ParentThreadUUID = %v, want nil", got.ParentThreadUUID)
	}
	if got.HostedSessionID != sessionID {
		t.Errorf("HostedSessionID = %q, want %q", got.HostedSessionID, sessionID)
	}
	if got.HostedFileName != "rollout-root.jsonl" {
		t.Errorf("HostedFileName = %q, want %q", got.HostedFileName, "rollout-root.jsonl")
	}
}

func TestUpsertRollout_ChildWithParent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	parent := rootUUID
	if err := store.UpsertRollout(ctx, userID, makeParams(childUUID, sessionID, "child.jsonl", &parent)); err != nil {
		t.Fatalf("UpsertRollout: %v", err)
	}

	got, err := store.GetRollout(ctx, userID, childUUID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.ParentThreadUUID == nil || *got.ParentThreadUUID != rootUUID {
		t.Errorf("ParentThreadUUID = %v, want %q", got.ParentThreadUUID, rootUUID)
	}
}

func TestUpsertRollout_IdempotentRepeat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	p := makeParams(rootUUID, sessionID, "root.jsonl", nil)
	if err := store.UpsertRollout(ctx, userID, p); err != nil {
		t.Fatalf("first UpsertRollout: %v", err)
	}
	if err := store.UpsertRollout(ctx, userID, p); err != nil {
		t.Fatalf("second UpsertRollout: %v", err)
	}

	var count int
	row := store.conn().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM codex_rollouts WHERE user_id = $1 AND thread_uuid = $2",
		userID, rootUUID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("row count after idempotent upsert = %d, want 1", count)
	}
}

func TestUpsertRollout_FirstWriteWinsOnParent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// First write: parent = rootUUID
	first := makeParams(childUUID, sessionID, "child.jsonl", ptr(rootUUID))
	if err := store.UpsertRollout(ctx, userID, first); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Second write: parent = siblingUUID (different). Must be preserved.
	second := makeParams(childUUID, sessionID, "child.jsonl", ptr(siblingUUID))
	if err := store.UpsertRollout(ctx, userID, second); err != nil {
		t.Fatalf("second: %v", err)
	}

	got, err := store.GetRollout(ctx, userID, childUUID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.ParentThreadUUID == nil || *got.ParentThreadUUID != rootUUID {
		t.Errorf("ParentThreadUUID = %v, want %q (first-write-wins)", got.ParentThreadUUID, rootUUID)
	}
}

func TestUpsertRollout_ParentNilThenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// First write: parent unknown (nil). Then learn parent on a later chunk.
	if err := store.UpsertRollout(ctx, userID, makeParams(childUUID, sessionID, "child.jsonl", nil)); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := store.UpsertRollout(ctx, userID, makeParams(childUUID, sessionID, "child.jsonl", ptr(rootUUID))); err != nil {
		t.Fatalf("second: %v", err)
	}

	got, err := store.GetRollout(ctx, userID, childUUID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.ParentThreadUUID == nil || *got.ParentThreadUUID != rootUUID {
		t.Errorf("ParentThreadUUID = %v, want %q (nil -> set must succeed)", got.ParentThreadUUID, rootUUID)
	}
}

func TestUpsertRollout_NonEmptyPreservedOverEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// First write: model populated.
	first := makeParams(rootUUID, sessionID, "root.jsonl", nil)
	first.Model = "gpt-5"
	first.CWD = "/home/u/project"
	if err := store.UpsertRollout(ctx, userID, first); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Second write: model/cwd empty (CLI may not include them in later chunks).
	second := makeParams(rootUUID, sessionID, "root.jsonl", nil)
	if err := store.UpsertRollout(ctx, userID, second); err != nil {
		t.Fatalf("second: %v", err)
	}

	got, err := store.GetRollout(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.Model != "gpt-5" {
		t.Errorf("Model = %q, want %q (empty must not overwrite non-empty)", got.Model, "gpt-5")
	}
	if got.CWD != "/home/u/project" {
		t.Errorf("CWD = %q, want %q", got.CWD, "/home/u/project")
	}
}

func TestUpsertRollout_EmptyOverwrittenByNonEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	first := makeParams(rootUUID, sessionID, "root.jsonl", nil) // model empty
	if err := store.UpsertRollout(ctx, userID, first); err != nil {
		t.Fatalf("first: %v", err)
	}

	second := first
	second.Model = "gpt-5"
	if err := store.UpsertRollout(ctx, userID, second); err != nil {
		t.Fatalf("second: %v", err)
	}

	got, err := store.GetRollout(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.Model != "gpt-5" {
		t.Errorf("Model = %q, want %q (non-empty must overwrite empty)", got.Model, "gpt-5")
	}
}

func TestUpsertRollout_UpdatedAtAdvances(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	p := makeParams(rootUUID, sessionID, "root.jsonl", nil)
	if err := store.UpsertRollout(ctx, userID, p); err != nil {
		t.Fatalf("first: %v", err)
	}
	r1, _ := store.GetRollout(ctx, userID, rootUUID)
	first := r1.UpdatedAt

	time.Sleep(20 * time.Millisecond) // Postgres NOW() resolution > sub-µs but be safe

	if err := store.UpsertRollout(ctx, userID, p); err != nil {
		t.Fatalf("second: %v", err)
	}
	r2, _ := store.GetRollout(ctx, userID, rootUUID)
	if !r2.UpdatedAt.After(first) {
		t.Errorf("updated_at did not advance: first=%v second=%v", first, r2.UpdatedAt)
	}
}

func TestUpsertRollout_RejectsMissingSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, _, ctx := setupCodexEnv(t)

	bogusSession := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	err := store.UpsertRollout(ctx, userID, makeParams(rootUUID, bogusSession, "x.jsonl", nil))
	if err == nil {
		t.Fatal("expected FK violation when hosted_session_id does not exist, got nil")
	}
}

func TestUpsertRollout_CrossUserNamespacing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	userA := testutil.CreateTestUser(t, env, "a@test.com", "A")
	userB := testutil.CreateTestUser(t, env, "b@test.com", "B")
	sessA := testutil.CreateTestSessionWithProvider(t, env, userA.ID, "ext-a", "codex")
	sessB := testutil.CreateTestSessionWithProvider(t, env, userB.ID, "ext-b", "codex")
	store := &Store{DB: env.DB}
	ctx := context.Background()

	// Same thread UUID for both users.
	if err := store.UpsertRollout(ctx, userA.ID, makeParams(rootUUID, sessA, "a.jsonl", nil)); err != nil {
		t.Fatalf("A upsert: %v", err)
	}
	if err := store.UpsertRollout(ctx, userB.ID, makeParams(rootUUID, sessB, "b.jsonl", nil)); err != nil {
		t.Fatalf("B upsert: %v", err)
	}

	gotA, err := store.GetRollout(ctx, userA.ID, rootUUID)
	if err != nil {
		t.Fatalf("A get: %v", err)
	}
	if gotA.HostedSessionID != sessA {
		t.Errorf("A HostedSessionID = %q, want %q", gotA.HostedSessionID, sessA)
	}

	gotB, err := store.GetRollout(ctx, userB.ID, rootUUID)
	if err != nil {
		t.Fatalf("B get: %v", err)
	}
	if gotB.HostedSessionID != sessB {
		t.Errorf("B HostedSessionID = %q, want %q", gotB.HostedSessionID, sessB)
	}
}

func TestGetRollout_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, _, ctx := setupCodexEnv(t)

	_, err := store.GetRollout(ctx, userID, rootUUID)
	if !errors.Is(err, db.ErrRolloutNotFound) {
		t.Errorf("err = %v, want ErrRolloutNotFound", err)
	}
}

func TestGetRollout_OtherUserHidden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	userA := testutil.CreateTestUser(t, env, "a@test.com", "A")
	userB := testutil.CreateTestUser(t, env, "b@test.com", "B")
	sessA := testutil.CreateTestSessionWithProvider(t, env, userA.ID, "ext-a", "codex")
	store := &Store{DB: env.DB}
	ctx := context.Background()

	if err := store.UpsertRollout(ctx, userA.ID, makeParams(rootUUID, sessA, "a.jsonl", nil)); err != nil {
		t.Fatalf("A upsert: %v", err)
	}

	_, err := store.GetRollout(ctx, userB.ID, rootUUID)
	if !errors.Is(err, db.ErrRolloutNotFound) {
		t.Errorf("user B reading A's row got err=%v, want ErrRolloutNotFound", err)
	}
}

func TestListSubtree_LinearChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// root -> child -> grandchild
	mustUpsert(t, store, userID, makeParams(rootUUID, sessionID, "r.jsonl", nil))
	time.Sleep(5 * time.Millisecond)
	mustUpsert(t, store, userID, makeParams(childUUID, sessionID, "c.jsonl", ptr(rootUUID)))
	time.Sleep(5 * time.Millisecond)
	mustUpsert(t, store, userID, makeParams(grandchildUUID, sessionID, "g.jsonl", ptr(childUUID)))

	got, err := store.ListSubtree(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("ListSubtree: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	want := []string{rootUUID, childUUID, grandchildUUID}
	for i, r := range got {
		if r.ThreadUUID != want[i] {
			t.Errorf("got[%d].ThreadUUID = %q, want %q (ordered by created_at ASC)",
				i, r.ThreadUUID, want[i])
		}
	}
}

func TestListSubtree_Branching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// root with two children
	mustUpsert(t, store, userID, makeParams(rootUUID, sessionID, "r.jsonl", nil))
	mustUpsert(t, store, userID, makeParams(childUUID, sessionID, "c1.jsonl", ptr(rootUUID)))
	mustUpsert(t, store, userID, makeParams(siblingUUID, sessionID, "c2.jsonl", ptr(rootUUID)))

	got, err := store.ListSubtree(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("ListSubtree: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	seen := map[string]bool{}
	for _, r := range got {
		seen[r.ThreadUUID] = true
	}
	for _, u := range []string{rootUUID, childUUID, siblingUUID} {
		if !seen[u] {
			t.Errorf("missing %q from subtree", u)
		}
	}
}

func TestListSubtree_RootOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	mustUpsert(t, store, userID, makeParams(rootUUID, sessionID, "r.jsonl", nil))

	got, err := store.ListSubtree(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("ListSubtree: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ThreadUUID != rootUUID {
		t.Errorf("ThreadUUID = %q, want %q", got[0].ThreadUUID, rootUUID)
	}
}

func TestListSubtree_QueryMissingRootReturnsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// Only a child exists; its parent_thread_uuid is an orphan reference.
	mustUpsert(t, store, userID, makeParams(childUUID, sessionID, "c.jsonl", ptr(rootUUID)))

	got, err := store.ListSubtree(ctx, userID, rootUUID)
	if err != nil {
		t.Fatalf("ListSubtree(orphan parent): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (orphan parent must not appear)", len(got))
	}
}

func TestListSubtree_OrphanChildReachableViaOwnUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	// Child references a never-uploaded parent.
	mustUpsert(t, store, userID, makeParams(childUUID, sessionID, "c.jsonl", ptr(rootUUID)))

	got, err := store.ListSubtree(ctx, userID, childUUID)
	if err != nil {
		t.Fatalf("ListSubtree: %v", err)
	}
	if len(got) != 1 || got[0].ThreadUUID != childUUID {
		t.Errorf("orphan child queried by own UUID: got %d rows, want 1 (the child)", len(got))
	}
}

func TestListSubtree_CascadeOnSessionDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, userID, sessionID, ctx := setupCodexEnv(t)

	mustUpsert(t, store, userID, makeParams(rootUUID, sessionID, "r.jsonl", nil))
	mustUpsert(t, store, userID, makeParams(childUUID, sessionID, "c.jsonl", ptr(rootUUID)))

	if _, err := store.conn().ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	var count int
	row := store.conn().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM codex_rollouts WHERE user_id = $1", userID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("rollouts after session delete = %d, want 0 (ON DELETE CASCADE)", count)
	}
}

func TestListSubtree_CascadeOnUserDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "del@test.com", "Del")
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "ext-del", "codex")
	store := &Store{DB: env.DB}
	ctx := context.Background()

	mustUpsert(t, store, user.ID, makeParams(rootUUID, sessionID, "r.jsonl", nil))

	if _, err := store.conn().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	row := store.conn().QueryRowContext(ctx, "SELECT COUNT(*) FROM codex_rollouts")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("rollouts after user delete = %d, want 0", count)
	}
}

func TestCodexRolloutsTable_SchemaShape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)

	// Table must be queryable.
	var n int
	if err := env.DB.QueryRow(env.Ctx, `SELECT COUNT(*) FROM codex_rollouts`).Scan(&n); err != nil {
		t.Fatalf("codex_rollouts table missing or unqueryable: %v", err)
	}

	// Both indexes must exist.
	for _, idxName := range []string{
		"idx_codex_rollouts_user_parent",
		"idx_codex_rollouts_session",
	} {
		var exists bool
		err := env.DB.QueryRow(env.Ctx,
			`SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'public' AND indexname = $1
			)`, idxName).Scan(&exists)
		if err != nil {
			t.Fatalf("pg_indexes lookup for %q: %v", idxName, err)
		}
		if !exists {
			t.Errorf("expected index %q to exist", idxName)
		}
	}
}

func mustUpsert(t *testing.T, s *Store, userID int64, p UpsertRolloutParams) {
	t.Helper()
	if err := s.UpsertRollout(context.Background(), userID, p); err != nil {
		t.Fatalf("UpsertRollout(%s): %v", p.ThreadUUID, err)
	}
}
