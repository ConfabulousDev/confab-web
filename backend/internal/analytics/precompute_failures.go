package analytics

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// Precompute failure quarantine (6kbe).
//
// The worker tracks consecutive failed attempts per (session, bucket) in
// session_precompute_failures. Each attempt is claimed (in_progress_since set)
// before it runs and settled after: success deletes the row, a failure bumps
// attempts, and quota-exceeded or shutdown releases the claim without counting.
// A claim still open at worker startup means the process died mid-attempt, so
// SweepCrashedAttempts counts it as a failure. After QuarantineAfterAttempts
// failures, quarantined_at is set and the bucket's Find* query excludes the
// session (notQuarantinedClause) until an admin releases it or an on-view
// compute for that bucket succeeds.

// Precompute buckets, one per worker bucket. Stored in
// session_precompute_failures.bucket (app-validated, no DB CHECK).
const (
	BucketRegularCards = "regular_cards"
	BucketSmartRecap   = "smart_recap"
	BucketSearchIndex  = "search_index"
	BucketSessionTitle = "session_title"
)

// QuarantineAfterAttempts is how many consecutive failed attempts in a bucket
// quarantine a session from that bucket. The worker makes one attempt per
// cycle, so at the default 30m poll this is about 2.5h of failures.
const QuarantineAfterAttempts = 5

// maxLastErrorBytes caps the stored last_error.
const maxLastErrorBytes = 1024

// crashedAttemptError is the last_error recorded for an attempt the worker
// never settled.
const crashedAttemptError = "worker exited during attempt"

var precomputeBuckets = []string{BucketRegularCards, BucketSmartRecap, BucketSearchIndex, BucketSessionTitle}

// IsPrecomputeBucket reports whether b is one of the four precompute buckets.
func IsPrecomputeBucket(b string) bool {
	return slices.Contains(precomputeBuckets, b)
}

// notQuarantinedClause returns a SQL predicate excluding sessions quarantined
// from bucket. sessionIDExpr must be a qualified outer column (sl.session_id,
// sessions.id): a bare id would resolve to the subquery's own table. bucket is
// inlined as a literal, so it must be one of the Bucket* constants.
func notQuarantinedClause(sessionIDExpr, bucket string) string {
	if !IsPrecomputeBucket(bucket) {
		panic(fmt.Sprintf("notQuarantinedClause: unknown bucket %q", bucket))
	}
	return fmt.Sprintf(`NOT EXISTS (SELECT 1 FROM session_precompute_failures f
				WHERE f.session_id = %s AND f.bucket = '%s' AND f.quarantined_at IS NOT NULL)`, sessionIDExpr, bucket)
}

// QuarantinedSession is one quarantined (session, bucket) row, joined with the
// session's owner and identity for the admin list.
type QuarantinedSession struct {
	SessionID     string
	UserID        int64
	ExternalID    string
	Provider      string // canonical (models.NormalizeProvider)
	Bucket        string
	Attempts      int
	LastError     string
	QuarantinedAt time.Time
}

// ClaimAttempt marks an attempt as in progress before the worker runs it. A
// new row starts at 0 attempts.
func (s *Store) ClaimAttempt(ctx context.Context, sessionID, bucket string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO session_precompute_failures (session_id, bucket, attempts, in_progress_since, updated_at)
		VALUES ($1, $2, 0, NOW(), NOW())
		ON CONFLICT (session_id, bucket) DO UPDATE
		SET in_progress_since = NOW(), updated_at = NOW()
	`, sessionID, bucket)
	return err
}

// RecordSuccess deletes the (session, bucket) row, resetting its attempts and
// releasing any quarantine.
func (s *Store) RecordSuccess(ctx context.Context, sessionID, bucket string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM session_precompute_failures WHERE session_id = $1 AND bucket = $2`, sessionID, bucket)
	return err
}

// RecordFailure counts one failed attempt, stores errMsg (truncated to 1 KB),
// and clears the in-progress claim. It inserts the row if it is missing (an
// on-view success may have deleted the claim mid-attempt). It returns the new
// attempt count and whether the session is now quarantined from bucket.
func (s *Store) RecordFailure(ctx context.Context, sessionID, bucket, errMsg string) (attempts int, quarantined bool, err error) {
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO session_precompute_failures AS f
			(session_id, bucket, attempts, last_error, in_progress_since, quarantined_at, updated_at)
		VALUES ($1, $2, 1, $3, NULL, CASE WHEN 1 >= $4 THEN NOW() END, NOW())
		ON CONFLICT (session_id, bucket) DO UPDATE SET
			attempts = f.attempts + 1,
			last_error = EXCLUDED.last_error,
			in_progress_since = NULL,
			quarantined_at = CASE WHEN f.attempts + 1 >= $4 THEN COALESCE(f.quarantined_at, NOW()) END,
			updated_at = NOW()
		RETURNING attempts, quarantined_at IS NOT NULL
	`, sessionID, bucket, validation.TruncateToByteLimit(errMsg, maxLastErrorBytes), QuarantineAfterAttempts,
	).Scan(&attempts, &quarantined)
	return attempts, quarantined, err
}

// ReleaseAttempt settles an attempt that must not count (quota exceeded,
// shutdown): it clears the claim and deletes the row if it has no failures.
func (s *Store) ReleaseAttempt(ctx context.Context, sessionID, bucket string) error {
	_, err := s.db.ExecContext(ctx, `
		WITH dropped AS (
			DELETE FROM session_precompute_failures
			WHERE session_id = $1 AND bucket = $2 AND attempts = 0
		)
		UPDATE session_precompute_failures
		SET in_progress_since = NULL, updated_at = NOW()
		WHERE session_id = $1 AND bucket = $2 AND attempts > 0
	`, sessionID, bucket)
	return err
}

// SweepCrashedAttempts counts every unsettled claim as a failed attempt. The
// worker runs it once at startup, before its first cycle: with a single worker
// process, any claim still open then belongs to a process that died
// mid-attempt (OOM, panic, hard kill). Returns the number of rows swept.
func (s *Store) SweepCrashedAttempts(ctx context.Context) (int, error) {
	return s.execCount(ctx, `
		UPDATE session_precompute_failures SET
			attempts = attempts + 1,
			last_error = $1,
			in_progress_since = NULL,
			quarantined_at = CASE WHEN attempts + 1 >= $2 THEN COALESCE(quarantined_at, NOW()) END,
			updated_at = NOW()
		WHERE in_progress_since IS NOT NULL
	`, crashedAttemptError, QuarantineAfterAttempts)
}

// ListQuarantined returns every quarantined (session, bucket), newest first.
func (s *Store) ListQuarantined(ctx context.Context) ([]QuarantinedSession, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.session_id, s.user_id, s.external_id, s.session_type, f.bucket,
			f.attempts, COALESCE(f.last_error, ''), f.quarantined_at
		FROM session_precompute_failures f
		JOIN sessions s ON s.id = f.session_id
		WHERE f.quarantined_at IS NOT NULL
		ORDER BY f.quarantined_at DESC, f.session_id, f.bucket
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QuarantinedSession
	for rows.Next() {
		var q QuarantinedSession
		var rawProvider string
		if err := rows.Scan(&q.SessionID, &q.UserID, &q.ExternalID, &rawProvider, &q.Bucket,
			&q.Attempts, &q.LastError, &q.QuarantinedAt); err != nil {
			return nil, err
		}
		q.Provider = models.NormalizeProvider(rawProvider)
		out = append(out, q)
	}
	return out, rows.Err()
}

// ReleaseQuarantined deletes one quarantined row and returns how many were
// deleted (0 if it was not quarantined).
func (s *Store) ReleaseQuarantined(ctx context.Context, sessionID, bucket string) (int, error) {
	return s.execCount(ctx, `
		DELETE FROM session_precompute_failures
		WHERE session_id = $1 AND bucket = $2 AND quarantined_at IS NOT NULL
	`, sessionID, bucket)
}

// ReleaseAllQuarantined deletes every quarantined row and returns the count.
// Rows still counting toward quarantine are kept.
func (s *Store) ReleaseAllQuarantined(ctx context.Context) (int, error) {
	return s.execCount(ctx,
		`DELETE FROM session_precompute_failures WHERE quarantined_at IS NOT NULL`)
}

// execCount runs query and returns the number of rows it affected.
func (s *Store) execCount(ctx context.Context, query string, args ...any) (int, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
