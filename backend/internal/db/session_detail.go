package db

// SessionDetailColumns is the canonical column list for loading a
// `SessionDetail` value from the `sessions` table joined to `users`.
//
// Both readers — the owner-only `db/session/GetSessionDetail` and the
// canonical-access `db/access/GetSessionDetailWithAccess` — share this
// list so a new field added to `SessionDetail` can't accidentally land
// in one path and not the other. (CF-347 missed this and shipped a
// production bug where Codex sessions ended up with `provider: ""` on
// the access path.)
//
// The order of columns MUST match the pointer order returned by
// `SessionDetailScanTargets`. Append-only at the end is the safe edit
// pattern; reordering forces every reader's Scan call to change in
// lockstep with this constant.
//
// The access path appends extra projection columns (e.g. `, u.status`)
// after this list and scans them by appending matching pointers to the
// targets slice. The shared portion is what stays in sync.
const SessionDetailColumns = `
	s.id, s.external_id, s.session_type, s.custom_title,
	s.suggested_session_title, s.summary, s.first_user_message,
	s.first_seen, s.cwd, s.transcript_path, s.git_info,
	s.last_sync_at, s.hostname, s.username, u.email`

// SessionDetailScanTargets returns the pointer arguments for scanning a
// row matching `SessionDetailColumns` in column order. The two row
// readers (owner-only and canonical-access) both call this so any future
// `SessionDetail` field addition forces a single edit here, propagating
// to every reader automatically.
//
// `gitInfoBytes` is scanned as raw bytes; callers are responsible for
// unmarshaling via `UnmarshalSessionGitInfo` after a successful Scan.
//
// Callers must invoke `models.NormalizeProvider` on `session.Provider`
// after scanning to map the legacy `'Claude Code'` display value to the
// canonical lowercase form (the permanent aliasing layer — see
// internal/models/provider.go for the OSS self-hosted rationale). This
// step is intentionally not folded into the helper so that the caller's
// error handling sits between Scan and the value mutation.
func SessionDetailScanTargets(session *SessionDetail, gitInfoBytes *[]byte) []any {
	return []any{
		&session.ID, &session.ExternalID, &session.Provider, &session.CustomTitle,
		&session.SuggestedSessionTitle, &session.Summary, &session.FirstUserMessage,
		&session.FirstSeen, &session.CWD, &session.TranscriptPath, gitInfoBytes,
		&session.LastSyncAt, &session.Hostname, &session.Username, &session.OwnerEmail,
	}
}
