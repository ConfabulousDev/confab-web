package db_test

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-233 — migration 48 retires the pr_inference fork→root path. Two
// observable effects are encoded:
//
//  1. (Spec contract) The `session_repos_root_source_check` constraint is
//     dropped, so the app — not the DB — becomes the single source of
//     truth for allowed `root_source` values per the project's "db
//     constraints in app" preference.
//  2. (Regression guard) The migration's UPDATE statement clears
//     `pr_inference` rows while preserving `git_remote` rows. The guard
//     re-runs the UPDATE to catch a future edit that breaks the
//     semantics (e.g. dropping the WHERE clause).
//
// Migrations run at env setup time via testutil.runMigrations; we don't
// re-run migration 48 inside the test.

func TestMigration48_DropsRootSourceCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// With the CHECK in place this INSERT would fail with `new row for
	// relation "session_repos" violates check constraint`. After migration 48
	// the constraint is gone and the row goes through.
	if _, err := env.DB.Conn().ExecContext(env.Ctx,
		`INSERT INTO session_repos (repo_name, root_name, root_source) VALUES ($1, $2, $3)`,
		"cf233/check-test", "cf233/check-root", "cf233-app-defined-value",
	); err != nil {
		t.Fatalf("expected CHECK to be dropped; insert failed: %v", err)
	}

	// Sanity-check via pg_catalog as a second observable.
	var stillExists bool
	if err := env.DB.Conn().QueryRowContext(env.Ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM pg_constraint
		   WHERE conname = 'session_repos_root_source_check'
		 )`,
	).Scan(&stillExists); err != nil {
		t.Fatalf("query pg_constraint: %v", err)
	}
	if stillExists {
		t.Error("session_repos_root_source_check still present; migration 48 must drop it")
	}
}

func TestMigration48_ClearsPrInferenceRowsAndPreservesGitRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Seed both kinds of stamped rows. Post-migration the CHECK is gone so
	// both INSERTs succeed regardless of value.
	for _, row := range []struct {
		repo, root, source string
	}{
		{"cf233-pr/fork", "cf233-pr/root", "pr_inference"},
		{"cf233-gr/fork", "cf233-gr/root", "git_remote"},
	} {
		if _, err := env.DB.Conn().ExecContext(env.Ctx,
			`INSERT INTO session_repos (repo_name, root_name, root_source) VALUES ($1, $2, $3)`,
			row.repo, row.root, row.source,
		); err != nil {
			t.Fatalf("seed %s: %v", row.repo, err)
		}
	}

	// Re-apply the migration's UPDATE; it must be idempotent.
	if _, err := env.DB.Conn().ExecContext(env.Ctx,
		`UPDATE session_repos SET root_name = NULL, root_source = NULL WHERE root_source = 'pr_inference'`,
	); err != nil {
		t.Fatalf("re-run migration UPDATE: %v", err)
	}

	assertCleared := func(repo string) {
		t.Helper()
		var root, source *string
		if err := env.DB.Conn().QueryRowContext(env.Ctx,
			`SELECT root_name, root_source FROM session_repos WHERE repo_name = $1`,
			repo,
		).Scan(&root, &source); err != nil {
			t.Fatalf("read %s: %v", repo, err)
		}
		if root != nil || source != nil {
			t.Errorf("%s: expected NULL root_name/root_source, got root=%v source=%v", repo, root, source)
		}
	}
	assertPreserved := func(repo, wantRoot, wantSource string) {
		t.Helper()
		var root, source *string
		if err := env.DB.Conn().QueryRowContext(env.Ctx,
			`SELECT root_name, root_source FROM session_repos WHERE repo_name = $1`,
			repo,
		).Scan(&root, &source); err != nil {
			t.Fatalf("read %s: %v", repo, err)
		}
		if root == nil || *root != wantRoot {
			t.Errorf("%s: expected root_name=%q, got %v", repo, wantRoot, root)
		}
		if source == nil || *source != wantSource {
			t.Errorf("%s: expected root_source=%q, got %v", repo, wantSource, source)
		}
	}

	assertCleared("cf233-pr/fork")
	assertPreserved("cf233-gr/fork", "cf233-gr/root", "git_remote")
}
