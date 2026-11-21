package access_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/access"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// xtrf: admins can view any session (read-only, redacted)
// =============================================================================

// adminCheck returns an access.AdminCheck with a fixed answer that counts how
// often it is called, so tests can assert the lazy contract.
func adminCheck(isAdmin bool, calls *int) access.AdminCheck {
	return func(context.Context) (bool, error) {
		if calls != nil {
			*calls++
		}
		return isAdmin, nil
	}
}

func TestAdminNonOwnerGetsAdminAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// No shares at all.
	info, err := store.GetSessionAccessType(context.Background(), sessionID, &adminUser.ID, adminCheck(true, nil))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessAdmin {
		t.Errorf("admin non-owner on unshared session: AccessType = %s, want %s", info.AccessType, db.SessionAccessAdmin)
	}
	if info.ShareID != nil {
		t.Errorf("admin access must not carry a ShareID, got %d", *info.ShareID)
	}

	// Shares exist, but none cover the admin (a recipient share for someone else).
	testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"someone@example.com"})
	info, err = store.GetSessionAccessType(context.Background(), sessionID, &adminUser.ID, adminCheck(true, nil))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessAdmin {
		t.Errorf("admin non-owner with uncovered shares: AccessType = %s, want %s", info.AccessType, db.SessionAccessAdmin)
	}
}

func TestNonAdminNonOwnerStillGetsNone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	for name, check := range map[string]access.AdminCheck{
		"nil check":     nil,
		"check says no": adminCheck(false, nil),
	} {
		info, err := store.GetSessionAccessType(context.Background(), sessionID, &viewer.ID, check)
		if err != nil {
			t.Fatalf("%s: GetSessionAccessType failed: %v", name, err)
		}
		if info.AccessType != db.SessionAccessNone {
			t.Errorf("%s: non-admin non-owner must get none, got %s", name, info.AccessType)
		}
	}
}

func TestAdminCheckErrorPropagates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	boom := errors.New("user lookup failed")
	_, err := store.GetSessionAccessType(context.Background(), sessionID, &viewer.ID,
		func(context.Context) (bool, error) { return false, boom })
	if !errors.Is(err, boom) {
		t.Errorf("admin check error must propagate, got %v", err)
	}
}

func TestAdminCheckIsLazyForOwnerAndShareAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")
	shareID := testutil.CreateTestShare(t, env, sessionID, false, nil, []string{recipient.Email})

	calls := 0
	// Admin who is also the owner: owner wins, check never consulted.
	info, err := store.GetSessionAccessType(context.Background(), sessionID, &owner.ID, adminCheck(true, &calls))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessOwner {
		t.Errorf("admin owner: AccessType = %s, want owner", info.AccessType)
	}

	// Admin who is also a recipient keeps share semantics (share_id).
	info, err = store.GetSessionAccessType(context.Background(), sessionID, &recipient.ID, adminCheck(true, &calls))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessRecipient {
		t.Errorf("admin recipient: AccessType = %s, want recipient", info.AccessType)
	}
	if info.ShareID == nil || *info.ShareID != shareID {
		t.Errorf("admin recipient: ShareID = %v, want %d", info.ShareID, shareID)
	}

	if calls != 0 {
		t.Errorf("admin check must not run when owner/share access already applies, ran %d times", calls)
	}
}

func TestAdminUnderShareAllGetsSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	env.DB.ShareAllSessions = true
	t.Cleanup(func() { env.DB.ShareAllSessions = false })
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	info, err := store.GetSessionAccessType(context.Background(), sessionID, &adminUser.ID, adminCheck(true, nil))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessSystem {
		t.Errorf("admin under share-all: AccessType = %s, want system (unchanged)", info.AccessType)
	}
}

func TestAnonymousViewerNeverAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	calls := 0
	info, err := store.GetSessionAccessType(context.Background(), sessionID, nil, adminCheck(true, &calls))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessNone {
		t.Errorf("anonymous viewer: AccessType = %s, want none", info.AccessType)
	}
	if calls != 0 {
		t.Errorf("admin check must not run for anonymous viewers, ran %d times", calls)
	}
}

func TestAdminDetailIsRedacted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	sessionID := testutil.CreateTestSessionWithGitInfoMap(t, env, owner.ID, "test-session", map[string]any{
		"repo_url": "https://user:secret@github.com/org/repo.git",
		"branch":   "main",
	})
	_, err := env.DB.Exec(env.Ctx,
		`UPDATE sessions SET cwd = '/home/owner/proj', hostname = 'owner-host', username = 'owner', transcript_path = '/home/owner/t.jsonl' WHERE id = $1`,
		sessionID)
	if err != nil {
		t.Fatalf("failed to seed PII: %v", err)
	}

	ctx := context.Background()
	check := adminCheck(true, nil)
	info, err := store.GetSessionAccessType(ctx, sessionID, &adminUser.ID, check)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	session, err := store.GetSessionDetailWithAccess(ctx, sessionID, &adminUser.ID, info, check)
	if err != nil {
		t.Fatalf("GetSessionDetailWithAccess failed: %v", err)
	}
	if session.CWD != nil || session.Hostname != nil || session.Username != nil || session.TranscriptPath != nil {
		t.Errorf("admin view must redact cwd/hostname/username/transcript_path, got cwd=%v host=%v user=%v path=%v",
			session.CWD, session.Hostname, session.Username, session.TranscriptPath)
	}
	if session.IsOwner == nil || *session.IsOwner {
		t.Errorf("admin view: is_owner must be false, got %v", session.IsOwner)
	}
	if session.OwnerEmail != owner.Email {
		t.Errorf("admin view: owner_email = %q, want %q", session.OwnerEmail, owner.Email)
	}
	if session.SharedByEmail == nil || *session.SharedByEmail != owner.Email {
		t.Errorf("admin view: shared_by_email = %v, want %q", session.SharedByEmail, owner.Email)
	}
	if session.AccessType != db.SessionAccessAdmin {
		t.Errorf("admin view: access_type = %q, want admin", session.AccessType)
	}
	gi, ok := session.GitInfo.(map[string]any)
	if !ok {
		t.Fatalf("admin view: git_info missing or wrong type: %#v", session.GitInfo)
	}
	// SanitizeGitInfoForSharing reduces repo_url to an owner/repo display
	// name, dropping the embedded credentials.
	if gi["repo_url"] != "org/repo" {
		t.Errorf("admin view: git_info must be sanitized to owner/repo, got %#v", gi)
	}
}

func TestAdminBypassesInactiveOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")
	testutil.CreateTestSystemShare(t, env, sessionID, nil)

	if err := (&dbuser.Store{DB: env.DB}).UpdateUserStatus(context.Background(), owner.ID, "inactive"); err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}
	ctx := context.Background()

	// Admin with no covering share → admin access, detail succeeds.
	_, err := env.DB.Exec(env.Ctx, `DELETE FROM session_shares WHERE session_id = $1`, sessionID)
	if err != nil {
		t.Fatalf("failed to drop shares: %v", err)
	}
	check := adminCheck(true, nil)
	info, err := store.GetSessionAccessType(ctx, sessionID, &adminUser.ID, check)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if _, err := store.GetSessionDetailWithAccess(ctx, sessionID, &adminUser.ID, info, check); err != nil {
		t.Errorf("admin on inactive owner's session must succeed, got %v", err)
	}

	// Admin reaching it through a system share also bypasses.
	testutil.CreateTestSystemShare(t, env, sessionID, nil)
	info, err = store.GetSessionAccessType(ctx, sessionID, &adminUser.ID, check)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if info.AccessType != db.SessionAccessSystem {
		t.Fatalf("admin with system share: AccessType = %s, want system", info.AccessType)
	}
	if _, err := store.GetSessionDetailWithAccess(ctx, sessionID, &adminUser.ID, info, check); err != nil {
		t.Errorf("admin via system share on inactive owner's session must succeed, got %v", err)
	}

	// Non-admin system-share viewer is still blocked.
	info, err = store.GetSessionAccessType(ctx, sessionID, &viewer.ID, adminCheck(false, nil))
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if _, err := store.GetSessionDetailWithAccess(ctx, sessionID, &viewer.ID, info, adminCheck(false, nil)); !errors.Is(err, db.ErrOwnerInactive) {
		t.Errorf("non-admin on inactive owner's session must get ErrOwnerInactive, got %v", err)
	}
}
