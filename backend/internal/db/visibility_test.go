package db_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-495: Integration tests for db.VisibleSessionsCTE — the cross-cutting
// visibility predicate shared by analytics, session-list pagination, and
// filter-options paths. Each test exercises the CTE as part of a real SQL
// query against the test postgres container, so we catch any drift between
// the helper string and the schema (column names, share table joins,
// expiration timestamps).

type visibleRow struct {
	id            string
	userID        int64
	ownerEmail    string
	accessType    string
	sharedByEmail *string
}

// runVisible runs the helper as a standalone CTE and collects rows for the
// given userID. We use the CTE directly so failures point at the helper, not
// at downstream callers.
func runVisible(t *testing.T, env *testutil.TestEnvironment, userID int64) []visibleRow {
	t.Helper()
	query := `WITH ` + db.VisibleSessionsCTE(env.DB.ShareAllSessions) + `
		SELECT id::text, user_id, owner_email, access_type, shared_by_email
		FROM visible_sessions ORDER BY id, access_type`
	rows, err := env.DB.Conn().QueryContext(context.Background(), query, userID)
	if err != nil {
		t.Fatalf("query visible_sessions: %v", err)
	}
	defer rows.Close()

	var out []visibleRow
	for rows.Next() {
		var r visibleRow
		if err := rows.Scan(&r.id, &r.userID, &r.ownerEmail, &r.accessType, &r.sharedByEmail); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iter: %v", err)
	}
	return out
}

func accessTypesFor(rows []visibleRow, id string) []string {
	var out []string
	for _, r := range rows {
		if r.id == id {
			out = append(out, r.accessType)
		}
	}
	sort.Strings(out)
	return out
}

func TestVisibleSessionsCTE_OwnedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@vis.test", "Bob")

	owned := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-owned", testutil.TestSessionFullOpts{Summary: "x"})
	_ = testutil.CreateTestSessionFull(t, env, bob.ID, "bob-owned", testutil.TestSessionFullOpts{Summary: "x"})

	rows := runVisible(t, env, alice.ID)
	if len(rows) != 1 {
		t.Fatalf("alice should see exactly 1 row (her own), got %d", len(rows))
	}
	r := rows[0]
	if r.id != owned {
		t.Errorf("row id = %s, want %s", r.id, owned)
	}
	if r.accessType != "owner" {
		t.Errorf("access_type = %q, want %q", r.accessType, "owner")
	}
	if r.sharedByEmail != nil {
		t.Errorf("shared_by_email = %v, want NULL for owner", *r.sharedByEmail)
	}
	if r.ownerEmail != "alice@vis.test" {
		t.Errorf("owner_email = %q, want alice@vis.test", r.ownerEmail)
	}
}

func TestVisibleSessionsCTE_PrivateShareGrantsAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@vis.test", "Bob")

	aliceSession := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-shared", testutil.TestSessionFullOpts{Summary: "x"})
	// Alice shares with Bob (private recipient share).
	testutil.CreateTestShare(t, env, aliceSession, false, nil, []string{bob.Email})

	rows := runVisible(t, env, bob.ID)
	if len(rows) != 1 {
		t.Fatalf("bob should see exactly 1 row (alice's via private share), got %d", len(rows))
	}
	r := rows[0]
	if r.id != aliceSession {
		t.Errorf("row id = %s, want %s", r.id, aliceSession)
	}
	if r.accessType != "private_share" {
		t.Errorf("access_type = %q, want %q", r.accessType, "private_share")
	}
	if r.sharedByEmail == nil || *r.sharedByEmail != "alice@vis.test" {
		t.Errorf("shared_by_email = %v, want alice@vis.test", r.sharedByEmail)
	}
	if r.ownerEmail != "alice@vis.test" {
		t.Errorf("owner_email = %q, want alice@vis.test", r.ownerEmail)
	}
}

func TestVisibleSessionsCTE_SystemShareGrantsAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@vis.test", "Bob")

	aliceSession := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-sys", testutil.TestSessionFullOpts{Summary: "x"})
	testutil.CreateTestSystemShare(t, env, aliceSession, nil)

	rows := runVisible(t, env, bob.ID)
	if len(rows) != 1 {
		t.Fatalf("bob should see exactly 1 row (alice's via system share), got %d", len(rows))
	}
	r := rows[0]
	if r.accessType != "system_share" {
		t.Errorf("access_type = %q, want %q", r.accessType, "system_share")
	}
	if r.sharedByEmail == nil || *r.sharedByEmail != "alice@vis.test" {
		t.Errorf("shared_by_email = %v, want alice@vis.test", r.sharedByEmail)
	}
}

func TestVisibleSessionsCTE_PrivatePlusSystemShareEmitsMultipleRows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@vis.test", "Bob")

	aliceSession := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-double", testutil.TestSessionFullOpts{Summary: "x"})
	// Alice shares the same session with bob via BOTH a private share and a
	// system share. UNION ALL must surface both rows (one access_type each)
	// so downstream pagination can priority-dedup (private_share wins).
	testutil.CreateTestShare(t, env, aliceSession, false, nil, []string{bob.Email})
	testutil.CreateTestSystemShare(t, env, aliceSession, nil)

	rows := runVisible(t, env, bob.ID)
	got := accessTypesFor(rows, aliceSession)
	want := []string{"private_share", "system_share"}
	if len(got) != len(want) {
		t.Fatalf("expected 2 visible rows for the same id (private + system), got %d (%v)", len(got), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("row %d access_type = %q, want %q", i, got[i], v)
		}
	}
}

func TestVisibleSessionsCTE_ShareAllMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	alice := testutil.CreateTestUser(t, env, "alice@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@vis.test", "Bob")

	aliceSession := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-sa", testutil.TestSessionFullOpts{Summary: "x"})
	bobSession := testutil.CreateTestSessionFull(t, env, bob.ID, "bob-sa", testutil.TestSessionFullOpts{Summary: "x"})

	rows := runVisible(t, env, bob.ID)
	if len(rows) != 2 {
		t.Fatalf("bob (with share-all) should see both sessions, got %d", len(rows))
	}

	byID := map[string]visibleRow{}
	for _, r := range rows {
		byID[r.id] = r
	}

	if r, ok := byID[bobSession]; !ok {
		t.Errorf("bob's own session missing from visible rows")
	} else {
		if r.accessType != "owner" {
			t.Errorf("bob's own session access_type = %q, want owner", r.accessType)
		}
		if r.sharedByEmail != nil {
			t.Errorf("bob's own session shared_by_email = %v, want NULL", *r.sharedByEmail)
		}
	}

	if r, ok := byID[aliceSession]; !ok {
		t.Errorf("alice's session missing from bob's share-all visible rows")
	} else {
		if r.accessType != "system_share" {
			t.Errorf("alice's session access_type for bob = %q, want system_share", r.accessType)
		}
		if r.sharedByEmail == nil || *r.sharedByEmail != "alice@vis.test" {
			t.Errorf("alice's session shared_by_email for bob = %v, want alice@vis.test", r.sharedByEmail)
		}
	}
}

func TestVisibleSessionsCTE_RevokedShareNoLongerVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice@vis.test", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@vis.test", "Bob")

	aliceSession := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-revoke", testutil.TestSessionFullOpts{Summary: "x"})

	// Alice shares with Bob initially — Bob sees it.
	past := time.Now().UTC().Add(-1 * time.Hour) // expired
	shareID := testutil.CreateTestShare(t, env, aliceSession, false, nil, []string{bob.Email})
	// Bump expires_at into the past to simulate revocation/expiry.
	if _, err := env.DB.Exec(env.Ctx, `UPDATE session_shares SET expires_at = $1 WHERE id = $2`, past, shareID); err != nil {
		t.Fatalf("set expired: %v", err)
	}

	rows := runVisible(t, env, bob.ID)
	if len(rows) != 0 {
		t.Fatalf("bob should see 0 rows after share expires, got %d", len(rows))
	}
}
